// Copyright (C) 2020 iDigitalFlame
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
	"strings"
	"sync"

	telegram "github.com/go-telegram-bot-api/telegram-bot-api"
)

const (
	helpMessage = `I'm sorry I don't recognize that command.
You can use the following commands:

/add <word> - Add a word to be swapped
/get <word> - Get the sticker assigned to the word
/remove <word> - Remove a swapped word

/list - List all your swapped words
/clear - Remove all your swapped words
/help - More information about me!`
	errorMessage = `Sorry I've seem to have encountered an error.

Please try again later.`
	helpMessageExtra = `
My name is SwapItBot!

My job is to swap out the messages you send with your assigned stickers!
Use the "/add <word>" to tell me a word and then send a Sticker for me to swap it with.
If I'm in a group that you're posting in, I will replace any of your set swap words.

I can also be used inline (inside the message box)!
Try this in any chat (I don't have to be in it) by entering @SwapItBot <word>

If you're an Admin of a group and would like to use me, have no fear!
I have some options that can set (per-group) to set limits on how many swaps I can do.
(These commands have to be used in the Group that you want to set the options in).

Here's the options that I have for Admins:
(All of these commands require Admin permissions, else I'll ignore them).

/swap_help
 - Show a help message.

/swap_options
 - Show the settings that you've configured for the Group.

/swap_limit <number of swaps (0 - 65535)>
 - Set a number of times I can perform a swap during the Timeout period. Set to zero to disable.

/swap_timeout <number of seconds (0 - 65535)>
 - Set a time (in seconds) that I can perform swaps that count twords the Limit count. Set to zero to disable.

/swap_delete <true|false|1|0|yes|no>
 - Determines if I will attempt to delete the swapped message (I can only delete if I have the permissions).

/swap_enable <true|false|1|0|yes|no>
 - Master switch to enable or disable swapping messages in this chat.

Please message my maintainers (@secfurry or @iDigitalFlame) for more info or questions!

My source code is located here: https://github.com/iDigitalFlame/swapper`
	helpMessageBasic = `Hello there, I'm SwapItBot!

I can swap or suggest stickers by a set word or parse!
You can call me inline  (inside the message box) by entering @SwapItBot <word>

If I'm added in a group chat, I can automatically swap out words with stickers!

I have the following commands:

/add <word> - Add a word to be swapped
/get <word> - Get the sticker assigned to the word
/remove <word> - Remove a swapped word

/list - List all your swapped words
/clear - Remove all your swapped words
/help - More information about me!`
)

var builders = sync.Pool{
	New: func() interface{} {
		return new(strings.Builder)
	},
}

var confirm struct{}

func (s *Swapper) list(x context.Context, i int) string {
	r, err := s.sql.QueryContext(x, "list", i)
	if err != nil {
		s.log.Error("Received an error when attemping to list the user swaps (UID: %d): %s!", i, err.Error())
		return errorMessage
	}
	var (
		c int
		n string
		b = builders.Get().(*strings.Builder)
	)
	for b.WriteString("You are currently swapping the words:\n"); r.Next(); {
		if err := r.Scan(&n); err != nil {
			s.log.Error("Received an error when attemping to scan the user swaps (UID: %d): %s!", i, err.Error())
			continue
		}
		if len(n) == 0 {
			continue
		}
		b.WriteString("- " + n + "\n")
		c++
	}
	r.Close()
	o := b.String()
	b.Reset()
	if builders.Put(b); c == 0 {
		return "You currently have no swapped words set."
	}
	return o
}
func (s *Swapper) clear(x context.Context, i int) string {

	if _, err := s.sql.ExecContext(x, "clear", i); err != nil {
		s.log.Error("Received an error when attemping to clear the user swaps (UID: %d): %s!", i, err.Error())
		return errorMessage
	}
	return "Sweet! I've cleared your swap list!"
}
func (s *Swapper) sticker(x context.Context, m *telegram.Message) string {
	v, ok := s.add[m.From.ID]
	if !ok {
		return helpMessage
	}
	delete(s.add, m.From.ID)
	if m.Sticker == nil {
		return "Sorry, but I require a Sticker.\n\nPlease invoke the command to try again."
	}
	if _, err := s.sql.ExecContext(x, "set_swap", m.From.ID, v, m.Sticker.FileID); err != nil {
		s.log.Error("Received an error when attemping to add a user swap (UID: %d): %s!", m.From.ID, err.Error())
		return errorMessage
	}
	return `Sweet! I added the sticker to the swap word "` + v + `"!`
}
func (s *Swapper) command(x context.Context, m *telegram.Message, o chan<- telegram.Chattable) {
	if m.Sticker != nil {
		o <- telegram.NewMessage(m.Chat.ID, s.sticker(x, m))
		return
	}
	_, ok := s.confirm[m.From.ID]
	if delete(s.confirm, m.From.ID); ok && strings.EqualFold(m.Text, "confirm") {
		o <- telegram.NewMessage(m.Chat.ID, s.clear(x, m.From.ID))
		return
	}
	if len(m.Text) <= 1 {
		o <- telegram.NewMessage(m.Chat.ID, helpMessage)
		return
	}
	var (
		l = strings.TrimSpace(m.Text[1:])
		d = strings.IndexByte(l, ' ')
	)
	if delete(s.add, m.From.ID); d == -1 {
		switch strings.ToLower(l) {
		case "help":
			o <- telegram.NewMessage(m.Chat.ID, helpMessageExtra)
			return
		case "list":
			o <- telegram.NewMessage(m.Chat.ID, s.list(x, m.From.ID))
			return
		case "clear":
			s.confirm[m.From.ID] = confirm
			o <- telegram.NewMessage(m.Chat.ID, `Please reply with "confirm" in order to clear your list.`)
			return
		case "start":
			o <- telegram.NewMessage(m.Chat.ID, helpMessageBasic)
			return
		}
		o <- telegram.NewMessage(m.Chat.ID, helpMessage)
		return
	}
	if d < 3 {
		o <- telegram.NewMessage(m.Chat.ID, helpMessage)
		return
	}
	v := strings.TrimSpace(l[d+1:])
	if len(v) == 0 {
		o <- telegram.NewMessage(m.Chat.ID, helpMessage)
		return
	}
	if len(v) > 16 || len(v) < 3 {
		o <- telegram.NewMessage(m.Chat.ID, "Sorry, but swapped words must be at least 3 characters and limited to a max of 16 characters!")
		return
	}
	switch strings.ToLower(l[:d]) {
	case "add":
		s.add[m.From.ID] = v
		o <- telegram.NewMessage(m.Chat.ID, `OK! Send me a sticker to swap for "`+v+`"`)
		return
	case "get":
		r, err := s.sql.QueryContext(x, "get_swap", m.From.ID, v)
		if err != nil {
			s.log.Error("Received an error when attemping to get a user swap (UID: %d): %s!", m.From.ID, err.Error())
			o <- telegram.NewMessage(m.Chat.ID, errorMessage)
			return
		}
		var n string
		for r.Next() {
			if err = r.Scan(&n); err != nil {
				break
			}
		}
		if r.Close(); err != nil {
			s.log.Error("Received an error when attemping to scan a user swap (UID: %d): %s!", m.From.ID, err.Error())
			o <- telegram.NewMessage(m.Chat.ID, errorMessage)
			return
		}
		if len(n) == 0 {
			o <- telegram.NewMessage(m.Chat.ID, `You don't have a sticker mapped for "`+v+"`!")
			return
		}
		o <- telegram.NewStickerShare(m.Chat.ID, n)
		return
	case "start":
		o <- telegram.NewMessage(m.Chat.ID, helpMessageBasic)
		return
	case "remove":
		if _, err := s.sql.ExecContext(x, "del_swap", m.From.ID, v); err != nil {
			s.log.Error("Received an error when attemping to del the user swap (UID: %d): %s!", m.From.ID, err.Error())
			o <- telegram.NewMessage(m.Chat.ID, errorMessage)
			return
		}
		o <- telegram.NewMessage(m.Chat.ID, `Sweet! I've remove the swap word "`+v+`"!`)
		return
	}
	o <- telegram.NewMessage(m.Chat.ID, helpMessage)
}
