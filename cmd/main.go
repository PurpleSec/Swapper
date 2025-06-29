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

package main

import (
	"flag"
	"os"

	"github.com/PurpleSec/swapper"
)

var buildVersion = "unknown"

const version = "v1.3.2"

const usage = `Sticker Swapper Telegram Bot ` + version + `
Purple Security (losynth.com/purple) 2021 - 2025

Usage:
  -h         Print this help menu.
  -V         Print version string and exit.
  -f <file>  Configuration file path.
  -d         Dump the default configuration and exit.
  -clear-all Clear the database of ALL DATA before starting up.
`

func main() {
	var (
		args             = flag.NewFlagSet("Sticker Swapper Telegram Bot "+version+"_"+buildVersion, flag.ExitOnError)
		file             string
		dump, empty, ver bool
	)
	args.Usage = func() {
		os.Stderr.WriteString(usage)
		os.Exit(2)
	}
	args.StringVar(&file, "f", "", "")
	args.BoolVar(&dump, "d", false, "")
	args.BoolVar(&ver, "V", false, "")
	args.BoolVar(&empty, "clear-all", false, "")

	if err := args.Parse(os.Args[1:]); err != nil {
		os.Stderr.WriteString(usage)
		os.Exit(2)
	}

	if ver {
		os.Stdout.WriteString("StickerSwap: " + version + "_" + buildVersion + "\n")
		os.Exit(0)
	}

	if len(file) == 0 && !dump {
		os.Stderr.WriteString(usage)
		os.Exit(2)
	}

	if dump {
		os.Stdout.WriteString(swapper.Defaults)
		os.Exit(0)
	}

	s, err := swapper.New(file, empty)
	if err != nil {
		os.Stdout.WriteString("Error: " + err.Error() + "!\n")
		os.Exit(1)
	}

	if err := s.Run(); err != nil {
		os.Stdout.WriteString("Error: " + err.Error() + "!\n")
		os.Exit(1)
	}
}
