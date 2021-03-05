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
	"strconv"
	"strings"

	telegram "github.com/go-telegram-bot-api/telegram-bot-api"
)

const (
	helpMessageAdmin = `As a group Admin, you can set some limits on me!

/swap_help
 - Show this help message.

/swap_options
 - Show the settings that you've configured for this Group.

/swap_limit <number of swaps (0 - 65535)>
 - Set a number of times I can perform a swap during the Timeout period. Set to zero to disable.

/swap_timeout <number of seconds (0 - 65535)>
 - Set a time (in seconds) that I can perform swaps that count twords the Limit count. Set to zero to disable.

/swap_delete <true|false|1|0|yes|no>
 - Determines if I will attempt to delete the swapped message (I can only delete if I have the permissions).

/swap_enable <true|false|1|0|yes|no>
 - Master switch to enable or disable swapping messages in this chat.`
	errorMessageAdmin = `Sorry I've seem to have encountered an error when changing that setting.

Please try again later.`
)

func stringMatchIndex(l int, s, m string) bool {
	if len(s) < l || len(s) < len(m) {
		return false
	}
	for i := 0; i < l; i++ {
		switch {
		case s[i] == m[i]:
		case m[i] > 96 && s[i]+32 == m[i]:
		case s[i] > 96 && m[i]+32 == s[i]:
		default:
			return false
		}
	}
	return true
}
func sendResponse(o chan<- telegram.Chattable, i int64, r int, s string) {
	n := telegram.NewMessage(i, s)
	n.ReplyToMessageID = r
	o <- n
}
func (s *Swapper) config(x context.Context, m *telegram.Message, o chan<- telegram.Chattable) {
	u, err := s.bot.GetChatMember(telegram.ChatConfigWithUser{ChatID: m.Chat.ID, UserID: m.From.ID})
	if err != nil {
		s.log.Error("Received an error during ChatMember lookup (GID: %d, UID: %d): %s!", m.Chat.ID, m.From.ID, err.Error())
		return
	}
	if u.Status != "administrator" && u.Status != "creator" {
		s.log.Debug("Non-admin user %q attempted an Admin command in GID %d!", m.From.String(), m.Chat.ID)
		return
	}
	l := strings.ToLower(strings.TrimSpace(m.Text[1:]))
	if l == "swap_help" {
		sendResponse(o, m.Chat.ID, m.MessageID, helpMessageAdmin)
		return
	}
	if l == "swap_options" {
		r, err := s.sql.QueryContext(x, "list_opt", m.Chat.ID)
		if err != nil {
			s.log.Error("Received an error when attemping to get group settings (GID: %d): %s!", m.Chat.ID, err.Error())
			sendResponse(o, m.Chat.ID, m.MessageID, errorMessageAdmin)
			return
		}
		var (
			e, b bool
			a, t int
		)
		for r.Next() {
			if err = r.Scan(&e, &a, &t, &b); err != nil {
				break
			}
		}
		if r.Close(); err != nil {
			s.log.Error("Received an error when attemping to scan group settings (GID: %d): %s!", m.Chat.ID, err.Error())
			sendResponse(o, m.Chat.ID, m.MessageID, errorMessageAdmin)
			return
		}
		sendResponse(o, m.Chat.ID, m.MessageID,
			"I have the following settings:\n\nSwapping Enabled: "+strconv.FormatBool(e)+"\nRemove Swapped: "+
				strconv.FormatBool(b)+"\nSwap Limit: "+strconv.Itoa(a)+"\nSwap Timeout: "+strconv.Itoa(t)+" seconds.",
		)
		return
	}
	d := strings.IndexByte(l, ' ')
	if d < 10 || len(l) <= d+1 {
		return
	}
	switch l[5:d] {
	case "limit":
		v, err := strconv.ParseUint(l[d+1:], 10, 16)
		if err != nil || v > 65536 {
			sendResponse(o, m.Chat.ID, m.MessageID, "Sorry I don't recognize that option value.\n\nThe correct usage should be \"/swap-limit <number of swaps (0 - 65535)>\"")
			return
		}
		if _, err := s.sql.ExecContext(x, "set_opt_limit", m.Chat.ID, v); err != nil {
			s.log.Error("Received an error when attemping to set the limit setting (GID: %d): %s!", m.Chat.ID, err.Error())
			sendResponse(o, m.Chat.ID, m.MessageID, errorMessageAdmin)
			return
		}
		s.log.Trace(`Admin %q set the "swap_limit" to %q setting for GID %d!`, m.From.String(), l[d+1:], m.Chat.ID)
		delete(s.limits, m.Chat.ID)
		sendResponse(o, m.Chat.ID, m.MessageID, `Awesome! I've updated the "swap_limit" setting to `+l[d+1:]+` swaps!`)
		return
	case "enable":
		var e bool
		switch l[d+1:] {
		case "1", "true", "t", "yes":
			e = true
		case "0", "false", "f", "no":
		default:
			sendResponse(o, m.Chat.ID, m.MessageID, "Sorry I don't recognize that option value.\n\nThe correct usage should be \"/swap-enabled <true|false|1|0|yes|no>\"")
			return
		}
		if _, err := s.sql.ExecContext(x, "set_opt_enable", m.Chat.ID, e); err != nil {
			s.log.Error("Received an error when attemping to set enable setting (GID: %d): %s!", m.Chat.ID, err.Error())
			sendResponse(o, m.Chat.ID, m.MessageID, errorMessageAdmin)
			return
		}
		s.log.Trace(`Admin %q set the "swap_enable" to %t setting for GID %d!`, m.From.String(), e, m.Chat.ID)
		sendResponse(o, m.Chat.ID, m.MessageID, `Sweet! I've updated the "swap_enable" setting to "`+strconv.FormatBool(e)+`"!`)
		return
	case "delete":
		var e bool
		switch l[d+1:] {
		case "1", "true", "t", "yes":
			e = true
		case "0", "false", "f", "no":
		default:
			sendResponse(o, m.Chat.ID, m.MessageID, "Sorry I don't recognize that option value.\n\nThe correct usage should be \"/swap-delete <true|false|1|0|yes|no>\"")
			return
		}
		if _, err := s.sql.ExecContext(x, "set_opt_delete", m.Chat.ID, e); err != nil {
			s.log.Error("Received an error when attemping to set the delete setting (GID: %d): %s!", m.Chat.ID, err.Error())
			sendResponse(o, m.Chat.ID, m.MessageID, errorMessageAdmin)
			return
		}
		s.log.Trace(`Admin %q set the "swap_delete" to %t setting for GID %d!`, m.From.String(), e, m.Chat.ID)
		sendResponse(o, m.Chat.ID, m.MessageID, `Sweet! I've updated the "swap_delete" setting to "`+strconv.FormatBool(e)+`"!`)
		return
	case "timeout":
		v, err := strconv.ParseUint(l[d+1:], 10, 16)
		if err != nil || v > 65536 {
			sendResponse(o, m.Chat.ID, m.MessageID, "Sorry I don't recognize that option value.\n\nThe correct usage should be \"/swap-timeout <number of seconds (0 - 65535)>\"")
			return
		}
		if _, err := s.sql.ExecContext(x, "set_opt_timeout", m.Chat.ID, v); err != nil {
			s.log.Error("Received an error when attemping to set timeout setting (GID: %d): %s!", m.Chat.ID, err.Error())
			sendResponse(o, m.Chat.ID, m.MessageID, errorMessageAdmin)
			return
		}
		s.log.Trace(`Admin %q set the "swap_timeout" to %q setting for GID %d!`, m.From.String(), l[d+1:], m.Chat.ID)
		delete(s.limits, m.Chat.ID)
		sendResponse(o, m.Chat.ID, m.MessageID, `Sweet! I've updated the "swap_timeout" setting to `+l[d+1:]+` seconds!`)
		return
	default:
	}
}
