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

var cleanStatements = []string{
	`DROP TABLES IF EXISTS Settings`,
	`DROP TABLES IF EXISTS Mappings`,
	`DROP PROCEDURE IF EXISTS GetSticker`,
	`DROP PROCEDURE IF EXISTS SetSticker`,
	`DROP PROCEDURE IF EXISTS SetSettingLimit`,
	`DROP PROCEDURE IF EXISTS SetSettingTimeout`,
	`DROP PROCEDURE IF EXISTS SetSettingEnabled`,
}

var setupStatements = []string{
	`CREATE TABLE IF NOT EXISTS Mappings(
		SwapID BIGINT(64) NOT NULL PRIMARY KEY AUTO_INCREMENT,
		UserID BIGINT(64) NOT NULL UNIQUE,
		Keyword VARCHAR(16) NOT NULL,
		StickerID VARCHAR(128) NOT NULL,
		StickerUID VARCHAR(128) NULL
	)`,
	`CREATE TABLE IF NOT EXISTS Settings(
		GroupID BIGINT(64) NOT NULL PRIMARY KEY,
		Amount INT(16) NOT NULL DEFAULT 5,
		Timeout INT(16) NOT NULL DEFAULT 5,
		Remove BOOLEAN NOT NULL DEFAULT TRUE,
		Enabled BOOLEAN NOT NULL DEFAULT TRUE
	)`,
	`ALTER TABLE Mappings ADD COLUMN IF NOT EXISTS (StickerUID VARCHAR(128) NULL)`,
	`DROP PROCEDURE IF EXISTS SetSticker`,
	`CREATE PROCEDURE IF NOT EXISTS SetSettingLimit(GID BIGINT(64), Amount INT(16))
	BEGIN
		SET @gid = COALESCE((SELECT GroupID FROM Settings WHERE GroupID = GID LIMIT 1), 0);
		IF @gid = 0 THEN
			INSERT INTO Settings(GroupID, Amount) VALUES(GID, Amount);
		ELSE
			UPDATE Settings SET Amount = Amount WHERE GroupID = GID;
		END IF;
	END;`,
	`CREATE PROCEDURE IF NOT EXISTS SetSettingDelete(GID BIGINT(64), Remove BOOLEAN)
	BEGIN
		SET @gid = COALESCE((SELECT GroupID FROM Settings WHERE GroupID = GID LIMIT 1), 0);
		IF @gid = 0 THEN
			INSERT INTO Settings(GroupID, Remove) VALUES(GID, Remove);
		ELSE
			UPDATE Settings SET Remove = Remove WHERE GroupID = GID;
		END IF;
	END;`,
	`CREATE PROCEDURE IF NOT EXISTS SetSettingTimeout(GID BIGINT(64), Timeout INT(16))
	BEGIN
		SET @gid = COALESCE((SELECT GroupID FROM Settings WHERE GroupID = GID LIMIT 1), 0);
		IF @gid = 0 THEN
			INSERT INTO Settings(GroupID, Timeout) VALUES(GID, Timeout);
		ELSE
			UPDATE Settings SET Timeout = Timeout WHERE GroupID = GID;
		END IF;
	END;`,
	`CREATE PROCEDURE IF NOT EXISTS SetSettingEnabled(GID BIGINT(64), Enabled BOOLEAN)
	BEGIN
		SET @gid = COALESCE((SELECT GroupID FROM Settings WHERE GroupID = GID LIMIT 1), 0);
		IF @gid = 0 THEN
			INSERT INTO Settings(GroupID, Enabled) VALUES(GID, Enabled);
		ELSE
			UPDATE Settings SET Enabled = Enabled WHERE GroupID = GID;
		END IF;
	END;`,
	`CREATE PROCEDURE IF NOT EXISTS GetSticker(User BIGINT(64), GID BIGINT(64), Word VARCHAR(16))
	BEGIN
		SET @gid = COALESCE((SELECT GroupID FROM Settings WHERE GroupID = GID LIMIT 1), 0);
		IF @gid = 0 THEN
			INSERT INTO Settings(GroupID) VALUES(GID);
		END IF;
		SELECT Enabled, Amount, Timeout, Remove, COALESCE((SELECT StickerID FROM Mappings WHERE UserID = User AND Keyword = Word LIMIT 1), "") As StickerID
			FROM Settings WHERE GroupID = GID;
	END;`,
	`CREATE PROCEDURE IF NOT EXISTS SetSticker(User BIGINT(64), Word VARCHAR(16), Sticker VARCHAR(128), SID VARCHAR(128))
	BEGIN
		SET @sid = COALESCE((SELECT SwapID FROM Mappings WHERE UserID = User AND Keyword = Word LIMIT 1), 0);
		IF @sid > 0 THEN
			UPDATE Mappings SET StickerID = Sticker, StickerUID = SID WHERE SwapID = @sid;
		ELSE
			INSERT INTO Mappings(UserID, StickerID, StickerUID, Keyword) VALUES(User, Sticker, SID, Word);
		END IF;
	END;`,
}

var queryStatements = map[string]string{
	"swap":             `CALL GetSticker(?, ?, ?)`,
	"list":             `SELECT Keyword FROM Mappings where UserID = ?`,
	"clear":            `DELETE FROM Mappings where UserID = ?`,
	"inline":           `SELECT StickerID FROM Mappings WHERE UserID = ? AND Keyword LIKE ?`,
	"get_swap":         `SELECT StickerID FROM Mappings WHERE UserID = ? AND Keyword = ?`,
	"set_swap":         `CALL SetSticker(?, ?, ?, ?)`,
	"del_swap":         `DELETE FROM Mappings WHERE UserID = ? AND Keyword = ?`,
	"list_opt":         `SELECT Enabled, Amount, Timeout, Remove FROM Settings WHERE GroupID = ?`,
	"inline_all":       `SELECT StickerID FROM Mappings WHERE UserID = ?`,
	"check_swap":       `SELECT Keyword FROM Mappings WHERE UserID = ? AND StickerUID = ?`,
	"set_opt_limit":    `CALL SetSettingLimit(?, ?)`,
	"set_opt_delete":   `CALL SetSettingDelete(?, ?)`,
	"set_opt_enable":   `CALL SetSettingEnabled(?, ?)`,
	"set_opt_timeout":  `CALL SetSettingTimeout(?, ?)`,
	"del_swap_sticker": `DELETE FROM Mappings WHERE UserID = ? AND StickerUID = ?`,
}
