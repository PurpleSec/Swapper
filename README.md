# Sticker Swapper Telegram Bot

Sticker Swapper Bot for Telegram

This is a bot for Telegram written in Golang, backed by a SQL database that is used for swapping or suggesting user
stickers via names or parses.

## Command Line Options

```[text]
Sticker Swapper Telegram Bot
Purple Security (losynth.com/purple) 2021 - 2025

Usage:
  -h              Print this help menu.
  -f <file>       Configuration file path.
  -d              Dump the default configuration and exit.
  -c              Clear the database of ALL DATA before starting up.
```

## Configuration Options

The default config can be dumped to Stdout using the '-d' command line flag.

```[json]
{
    "db": {
        "host": "tcp(localhost:3306)",
        "user": "swapper_user",
        "timeout": 180000000000,
        "password": "password",
        "database": "swapper_db"
    },
    "log": {
        "file": "swapper.log",
        "level": 2
    },
    "telegram_key": ""
}
```

The "telegram_key" can also be a string list that can be used to manage multiple
Telegram accounts.

[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/Z8Z4121TDS)
