// Copyright (C) 2021 - 2025 PurpleSec Team
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
	"strconv"
	"strings"
	"sync"
	"time"

	telegram "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (s *Swapper) check(i int64) bool {
	l, ok := s.limits[i]
	if !ok {
		return true
	}
	if l.max == 0 {
		return true
	}
	if time.Now().After(l.free) {
		l.count, l.free = 1, time.Now().Add(l.gap)
		return true
	}
	if l.count >= l.max {
		return false
	}
	l.count++
	return true
}
func (s *Swapper) update(i int64, a, t uint16) {
	l, ok := s.limits[i]
	if !ok {
		l = new(limit)
		s.limits[i] = l
	}
	l.gap, l.max = time.Duration(t)*time.Second, a
	if time.Now().After(l.free) {
		l.count, l.free = 0, time.Now().Add(l.gap)
	}
}
func (s *Swapper) inline(x context.Context, m *telegram.InlineQuery) []any {
	if len(m.Query) < 1 || len(m.Query) > 16 {
		return nil
	}
	var (
		r   *sql.Rows
		err error
	)
	if m.Query == "*" {
		r, err = s.sql.QueryContext(x, "inline_all", m.From.ID)
	} else {
		r, err = s.sql.QueryContext(x, "inline", m.From.ID, strings.TrimSpace(m.Query)+"%")
	}
	if err != nil {
		s.log.Error("Received an error attempting to get the inline sticker value for UID: %d: %s!", m.From.ID, err.Error())
		return nil
	}
	var (
		v string
		o []any
	)
	for i := 0; r.Next() && i < 50; i++ {
		if err = r.Scan(&v); err != nil {
			break
		}
		o = append(o, telegram.NewInlineQueryResultCachedSticker(m.ID+"res"+strconv.Itoa(i), v, ""))
	}
	if r.Close(); err != nil {
		s.log.Error("Received an error attempting to scan the inline sticker value for UID: %d: %s!", m.From.ID, err.Error())
		return nil
	}
	if len(o) == 0 {
		return nil
	}
	s.log.Trace(`Found an inline swap match "%s" by %s!`, v, m.From.String())
	return o
}
func (c *container) send(x context.Context, s *Swapper, g *sync.WaitGroup, o <-chan telegram.Chattable) {
	s.log.Debug("Starting Telegram sender thread..")
	for g.Add(1); ; {
		select {
		case n := <-o:
			if _, err := c.bot.Send(n); err != nil {
				s.log.Error(`Error sending Telegram message to chat: %s!`, err.Error())
			}
		case <-x.Done():
			s.log.Debug("Stopping Telegram sender thread.")
			g.Done()
			return
		}
	}
}
func (c *container) swap(x context.Context, s *Swapper, m *telegram.Message, o chan<- telegram.Chattable) {
	if m.From.IsBot || len(m.Text) == 0 || len(m.Text) < 3 || len(m.Text) > 16 || m.Text[0] == '/' || m.Text[0] < 33 {
		return
	}
	if !s.check(m.Chat.ID) {
		s.log.Trace("Hit a timeout limit on GID %d!", m.Chat.ID)
		return
	}
	var (
		v    string
		e, p bool
		a, t uint16
	)
	r, err := s.sql.QueryContext(x, "swap", m.From.ID, m.Chat.ID, strings.TrimSpace(m.Text))
	if err != nil {
		s.log.Error("Received an error attempting to get the sticker value for GID %d, UID: %d: %s!", m.Chat.ID, m.From.ID, err.Error())
		return
	}
	for r.Next() {
		if err = r.Scan(&e, &a, &t, &p, &v); err != nil {
			break
		}
	}
	if r.Close(); err != nil {
		s.log.Error("Received an error attempting to scan the sticker value for GID %d, UID: %d: %s!", m.Chat.ID, m.From.ID, err.Error())
		return
	}
	if s.update(m.Chat.ID, a, t); !e || len(v) == 0 {
		return
	}
	s.log.Trace(`Found a swap match "%s" by "%s"!`, v, m.From.String())
	n := telegram.NewSticker(m.Chat.ID, telegram.FileID(v))
	if m.ReplyToMessage != nil {
		n.ReplyToMessageID = m.ReplyToMessage.MessageID
	}
	if p {
		s.log.Trace("Attempting to delete the swapped message %d..", m.MessageID)
		if _, err = c.bot.Request(telegram.NewDeleteMessage(m.Chat.ID, m.MessageID)); err != nil {
			s.log.Warning("Received an error attempting to delete a message from GID %s: %s", m.Chat.ID, err.Error())
		}
	}
	if v = "@" + m.From.UserName; len(v) <= 1 {
		v = m.From.String()
	}
	o <- n
	o <- telegram.NewMessage(m.Chat.ID, "Swapped message from "+v)
}
func (c *container) receive(x context.Context, s *Swapper, g *sync.WaitGroup, o chan<- telegram.Chattable, r <-chan telegram.Update) {
	s.log.Debug("Starting Telegram receiver thread..")
	for g.Add(1); ; {
		select {
		case n := <-r:
			if n.InlineQuery != nil {
				k := telegram.InlineConfig{
					Results:       s.inline(x, n.InlineQuery),
					CacheTime:     180,
					IsPersonal:    true,
					InlineQueryID: n.InlineQuery.ID,
				}
				if len(k.Results) == 0 {
					k.SwitchPMParameter, k.SwitchPMText = "new", "Click here to add some Stickers!"
				}
				if _, err := c.bot.Request(k); err != nil {
					s.log.Error("Received error during inline query response: %s!", err.Error())
				}
				break
			}
			if n.Message == nil || n.Message.Chat == nil || (len(n.Message.Text) == 0 && n.Message.Sticker == nil) {
				break
			}
			if n.Message.Chat.IsPrivate() {
				s.log.Trace("Received a possible command/sticker from %s!", n.Message.From.String())
				s.command(x, n.Message, o)
				break
			}
			if n.Message.From.IsBot {
				break
			}
			if len(n.Message.Text) > 6 && n.Message.Text[0] == '/' && stringMatchIndex(6, n.Message.Text, "/swap_") {
				s.log.Trace("Received a possible command message from %s!", n.Message.From.String())
				c.config(x, s, n.Message, o)
				break
			}
			c.swap(x, s, n.Message, o)
		case <-x.Done():
			s.log.Debug("Stopping Telegram receiver thread.")
			g.Done()
			return
		}
	}
}
