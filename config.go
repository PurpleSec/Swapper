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
	"time"

	// Import for the Golang MySQL driver
	_ "github.com/go-sql-driver/mysql"
)

// Defaults is a string representation of a JSON formatted default configuration for a Watcher instance.
const Defaults = `{
	"db": {
		"host": "tcp(localhost:3306)",
		"user": "swapper_user",
		"timeout": 180000000000,
		"password": "password",
		"database": "swapper_db"
	},
	"log": {
		"file": "watcher.log",
		"level": 2
	},
	"telegram_key": ""
}
`

type log struct {
	File  string `json:"file"`
	Level int    `json:"level"`
}
type limit struct {
	free  time.Time
	gap   time.Duration
	max   uint16
	count uint16
}
type errval struct {
	e error
	s string
}
type config struct {
	Database database `json:"db"`
	Telegram string   `json:"telegram_key"`
	Log      log      `json:"log"`
}
type database struct {
	Name     string        `json:"database"`
	Server   string        `json:"host"`
	Username string        `json:"user"`
	Password string        `json:"password"`
	Timeout  time.Duration `json:"timeout"`
}

func (e errval) Error() string {
	if e.e == nil {
		return e.s
	}
	return e.s + ": " + e.e.Error()
}
func (c *config) check() error {
	if len(c.Database.Name) == 0 {
		return &errval{s: "missing database name"}
	}
	if len(c.Database.Server) == 0 {
		return &errval{s: "missing database server"}
	}
	if len(c.Database.Username) == 0 {
		return &errval{s: "missing database username"}
	}
	if c.Database.Timeout == 0 {
		c.Database.Timeout = time.Minute * 3
	}
	return nil
}
