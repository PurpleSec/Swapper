// Copyright (C) 2021 - 2022 PurpleSec Team
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
	"strconv"
	"strings"
	"sync"
	"time"

	telegram "github.com/go-telegram-bot-api/telegram-bot-api"
)

// Shamelessly copied from:
// https://github.com/go-telegram-bot-api/telegram-bot-api/blob/05e04b526c693e3e104feaa0be23611836af3dcc/helpers.go#L575
//
// Since the standard API doesn't have it and the newer 'v5' versions are wonky
// as hell.
type sticker struct {
	InputMessageContent interface{} `json:"input_message_content,omitempty"`
	ReplyMarkup         interface{} `json:"reply_markup,omitempty"`
	Type                string      `json:"type"`
	Title               string      `json:"title"`
	ParseMode           string      `json:"parse_mode"`
	StickerID           string      `json:"sticker_file_id"`
	ID                  string      `json:"id"`
}

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
func (s *Swapper) inline(x context.Context, m *telegram.InlineQuery) []interface{} {
	if len(m.Query) < 3 || len(m.Query) > 16 {
		return nil
	}
	r, err := s.sql.QueryContext(x, "inline", m.From.ID, strings.TrimSpace(m.Query)+"%")
	if err != nil {
		s.log.Error("Received an error attempting to get the inline sticker value for UID: %d: %s!", m.From.ID, err.Error())
		return nil
	}
	var (
		v string
		o []interface{}
	)
	for i := 0; r.Next(); i++ {
		if err = r.Scan(&v); err != nil {
			break
		}
		o = append(o, sticker{ID: m.ID + "res" + strconv.Itoa(i), Type: "sticker", Title: m.Query, StickerID: v})
	}
	if r.Close(); err != nil {
		s.log.Error("Received an error attempting to scan the inline sticker value for UID: %d: %s!", m.From.ID, err.Error())
		return nil
	}
	if len(o) == 0 {
		return nil
	}
	s.log.Trace("Found an inline swap match %q by %s!", v, m.From.String())
	return o
}
func (s *Swapper) send(x context.Context, g *sync.WaitGroup, o <-chan telegram.Chattable) {
	s.log.Debug("Starting Telegram sender thread...")
	for g.Add(1); ; {
		select {
		case n := <-o:
			if _, err := s.bot.Send(n); err != nil {
				s.log.Error(`Error sending Telegram message to chat: %s!`, err.Error())
			}
		case <-x.Done():
			s.log.Debug("Stopping Telegram sender thread.")
			g.Done()
			return
		}
	}
}
func (s *Swapper) swap(x context.Context, m *telegram.Message, o chan<- telegram.Chattable) {
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
	s.log.Trace("Found a swap match %q by %s!", v, m.From.String())
	n := telegram.NewStickerShare(m.Chat.ID, v)
	if m.ReplyToMessage != nil {
		n.ReplyToMessageID = m.ReplyToMessage.MessageID
	}
	if p {
		s.log.Trace("Attempting to delete the swapped message %d...", m.MessageID)
		if _, err = s.bot.DeleteMessage(telegram.DeleteMessageConfig{ChatID: m.Chat.ID, MessageID: m.MessageID}); err != nil {
			s.log.Warning("Received an error attempting to delete a message from GID %s: %s", m.Chat.ID, err.Error())
		}
	}
	if v = "@" + m.From.UserName; len(v) <= 1 {
		v = m.From.String()
	}
	o <- n
	o <- telegram.NewMessage(m.Chat.ID, "Swapped message from "+v)
}
func (s *Swapper) receive(x context.Context, g *sync.WaitGroup, o chan<- telegram.Chattable, r <-chan telegram.Update) {
	s.log.Debug("Starting Telegram receiver thread...")
	for g.Add(1); ; {
		select {
		case n := <-r:
			if n.InlineQuery != nil {
				c := telegram.InlineConfig{
					Results:       s.inline(x, n.InlineQuery),
					CacheTime:     30,
					IsPersonal:    true,
					InlineQueryID: n.InlineQuery.ID,
				}
				if len(c.Results) == 0 {
					c.SwitchPMParameter, c.SwitchPMText = "new", "Click here to add some Stickers!"
				}
				if _, err := s.bot.AnswerInlineQuery(c); err != nil {
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
				s.config(x, n.Message, o)
				break
			}
			s.swap(x, n.Message, o)
		case <-x.Done():
			s.log.Debug("Stopping Telegram receiver thread.")
			g.Done()
			return
		}
	}
}
