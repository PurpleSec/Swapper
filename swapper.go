// Copyright (C) 2021 - 2023 PurpleSec Team
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
	"errors"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	"github.com/PurpleSec/logx"
	"github.com/PurpleSec/mapper"

	telegram "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Swapper is a struct that contains the threads and config values that can be
// used to run the StickerSwap Telegram bot.
//
// Use the 'NewSwapper' function to properly create a Swapper.
type Swapper struct {
	log     logx.Log
	sql     *mapper.Map
	add     map[int64]string
	del     map[int64]struct{}
	lock    sync.RWMutex
	cancel  context.CancelFunc
	limits  map[int64]*limit
	confirm map[int64]struct{}
	bots    []*container
}
type container struct {
	ch  chan telegram.Chattable
	bot *telegram.BotAPI
}

func (c *container) stop() {
	c.bot.StopReceivingUpdates()
	close(c.ch)
	c.bot, c.ch = nil, nil
}

// Run will start the main Swapper process and all associated threads. This
// function will block until an interrupt signal is received.
//
// This function returns any errors that occur during shutdown.
func (s *Swapper) Run() error {
	var (
		o = make(chan os.Signal, 1)
		x context.Context
		g sync.WaitGroup
	)
	signal.Notify(o, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	x, s.cancel = context.WithCancel(context.Background())
	s.log.Info("Swapper Telegram Bot Started, spinning up threads..")
	for i := range s.bots {
		s.log.Debug("Starting bot %d..", i)
		s.bots[i].start(x, s, &g)
	}
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
	for i := range s.bots {
		s.log.Debug("Stopping bot %d..", i)
		s.bots[i].stop()
	}
	g.Wait()
	close(o)
	return s.sql.Close()
}

// New returns a new Swapper instance based on the passed config file path. This function will preform any
// setup steps needed to start the Swapper. Once complete, use the 'Run' function to actually start the Swapper.
// This function allows for specifying the option to clear the database before starting.
func New(s string, empty bool) (*Swapper, error) {
	var c config
	j, err := os.ReadFile(s)
	if err != nil {
		return nil, errors.New(`reading config "` + s + `": ` + err.Error())
	}
	if err = json.Unmarshal(j, &c); err != nil {
		return nil, errors.New(`parsing config "` + s + `": ` + err.Error())
	}
	if err = c.check(); err != nil {
		return nil, err
	}
	l := logx.Multiple(logx.Console(logx.Level(c.Log.Level)))
	if len(c.Log.File) > 0 {
		f, err2 := logx.File(c.Log.File, logx.Append, logx.Level(c.Log.Level))
		if err2 != nil {
			return nil, errors.New(`log file "` + c.Log.File + `": ` + err2.Error())
		}
		l.Add(f)
	}
	z := make([]*container, 1, 4)
	if k := len(c.Telegram.e); k > 0 {
		z = append(z, make([]*container, k-1)...)
		for i := range c.Telegram.e {
			b, err := telegram.NewBotAPI(c.Telegram.e[i])
			if err != nil {
				return nil, errors.New("telegram key (" + strconv.Itoa(i) + ") login: " + err.Error())
			}
			z[i] = &container{bot: b}
		}
	} else {
		b, err := telegram.NewBotAPI(c.Telegram.s)
		if err != nil {
			return nil, errors.New("telegram login: " + err.Error())
		}
		z[0] = &container{bot: b}
	}
	if len(z) == 1 && z[0] == nil {
		return nil, errors.New("no telegram accounts")
	}
	d, err := sql.Open(
		"mysql",
		c.Database.Username+":"+c.Database.Password+"@"+c.Database.Server+"/"+c.Database.Name+"?multiStatements=true&interpolateParams=true",
	)
	if err != nil {
		return nil, errors.New(`database connection "` + c.Database.Server + `": ` + err.Error())
	}
	if err = d.Ping(); err != nil {
		return nil, errors.New(`database connection "` + c.Database.Server + `": ` + err.Error())
	}
	m := mapper.New(d)
	if d.SetConnMaxLifetime(c.Database.Timeout); empty {
		if err = m.Batch(cleanStatements); err != nil {
			m.Close()
			return nil, errors.New("clean up: " + err.Error())
		}
	}
	if err = m.Batch(setupStatements); err != nil {
		m.Close()
		return nil, errors.New("database schema: " + err.Error())
	}
	if err = m.Extend(queryStatements); err != nil {
		m.Close()
		return nil, errors.New("database schema: " + err.Error())
	}
	return &Swapper{
		sql:     m,
		log:     l,
		add:     make(map[int64]string),
		del:     make(map[int64]struct{}),
		bots:    z,
		limits:  make(map[int64]*limit),
		confirm: make(map[int64]struct{}),
	}, nil
}
func (c *container) start(x context.Context, s *Swapper, g *sync.WaitGroup) {
	r := c.bot.GetUpdatesChan(telegram.UpdateConfig{})
	c.ch = make(chan telegram.Chattable, 128)
	go c.send(x, s, g, c.ch)
	go c.receive(x, s, g, c.ch, r)
}
