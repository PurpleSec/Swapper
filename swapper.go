// Copyright (C) 2021 PurpleSec Team
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
//

package swapper

import (
	"context"
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/PurpleSec/logx"
	"github.com/PurpleSec/mapper"

	telegram "github.com/go-telegram-bot-api/telegram-bot-api"
)

// Swapper is a struct that contains the threads and config values that can be used to run the StickerSwap Telegram
// bot. Use the 'NewSwapper' function to properly create a Swapper.
type Swapper struct {
	sql     *mapper.Map
	bot     *telegram.BotAPI
	log     logx.Log
	add     map[int]string
	cancel  context.CancelFunc
	limits  map[int64]*limit
	confirm map[int]struct{}
}

// Run will start the main Swapper process and all associated threads. This function will block until an
// interrupt signal is received. This function returns any errors that occur during shutdown.
func (s *Swapper) Run() error {
	r, err := s.bot.GetUpdatesChan(telegram.UpdateConfig{})
	if err != nil {
		s.sql.Close()
		return &errval{s: "could not get Telegram receiver", e: err}
	}
	var (
		o = make(chan os.Signal, 1)
		m = make(chan telegram.Chattable, 256)
		x context.Context
		g sync.WaitGroup
	)
	signal.Notify(o, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	x, s.cancel = context.WithCancel(context.Background())
	s.log.Info("Swapper Telegram Bot Started, spinning up threads...")
	go s.send(x, &g, m)
	go s.receive(x, &g, m, r)
	for {
		select {
		case <-o:
			goto cleanup
		case <-x.Done():
			goto cleanup
		}
	}
cleanup:
	signal.Stop(o)
	s.cancel()
	s.bot.StopReceivingUpdates()
	g.Wait()
	close(o)
	close(m)
	return s.sql.Close()
}

// New returns a new Swapper instance based on the passed config file path. This function will preform any
// setup steps needed to start the Swapper. Once complete, use the 'Run' function to actually start the Swapper.
// This function allows for specifying the option to clear the database before starting.
func New(s string, empty bool) (*Swapper, error) {
	var c config
	j, err := ioutil.ReadFile(s)
	if err != nil {
		return nil, &errval{s: `error reading config file "` + s + `"`, e: err}
	}
	if err := json.Unmarshal(j, &c); err != nil {
		return nil, &errval{s: `error parsing config file "` + s + `"`, e: err}
	}
	if err := c.check(); err != nil {
		return nil, err
	}
	l := logx.Multiple(logx.Console(logx.Level(c.Log.Level)))
	if len(c.Log.File) > 0 {
		f, err := logx.File(c.Log.File, logx.Append, logx.Level(c.Log.Level))
		if err != nil {
			return nil, &errval{s: `error setting up log file "` + c.Log.File + `"`, e: err}
		}
		l.Add(f)
	}
	b, err := telegram.NewBotAPI(c.Telegram)
	if err != nil {
		return nil, &errval{s: "login to Telegram failed", e: err}
	}
	d, err := sql.Open(
		"mysql",
		c.Database.Username+":"+c.Database.Password+"@"+c.Database.Server+"/"+c.Database.Name+"?multiStatements=true&interpolateParams=true",
	)
	if err != nil {
		return nil, &errval{s: `database connection "` + c.Database.Server + `" failed`, e: err}
	}
	if err = d.Ping(); err != nil {
		return nil, &errval{s: `database connection "` + c.Database.Server + `" failed`, e: err}
	}
	m := mapper.New(d)
	if d.SetConnMaxLifetime(c.Database.Timeout); empty {
		if err = m.Batch(cleanStatements); err != nil {
			m.Close()
			return nil, &errval{s: "could not clean up database schema", e: err}
		}
	}
	if err = m.Batch(setupStatements); err != nil {
		m.Close()
		return nil, &errval{s: "could not set up database schema", e: err}
	}
	if err = m.Extend(queryStatements); err != nil {
		m.Close()
		return nil, &errval{s: "could not set up database schema", e: err}
	}
	return &Swapper{
		sql:     m,
		bot:     b,
		log:     l,
		add:     make(map[int]string),
		limits:  make(map[int64]*limit),
		confirm: make(map[int]struct{}),
	}, nil
}
