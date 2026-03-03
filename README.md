# bot-wa-go

A Go-based WhatsApp bot powered by [`whatsmeow`](https://github.com/tulir/whatsmeow), featuring interactive QR/pairing login, plugin-based commands, MongoDB storage, media/downloader tools, and an economy + game system.

## Features
- Interactive startup: run once and choose `QR` or `Pairing` in terminal.
- Multi-prefix support (owner configurable): default `.`, `!`, `/`, `?`, `#`.
- Plugin architecture by domain: `owner`, `group`, `media`, `downloader`, `game`, `general`.
- MongoDB-backed persistent data:
  - `settings`, `groups`, `users`, `tickets` collections.
- Multi-owner model:
  - `owner_number` in `config.json` = super owners.
  - Additional owners can be managed via commands.
- Media tools: `sticker`, `toimg`, `toaudio`, `tovn`, `qc`, `brat`, `upscale`.
- Downloader tools: TikTok and Instagram.
- Group moderation/features: `antilink`, `welcome`, `goodbye`, `tagall`, `hidetag`.
- Economy system: profile, XP/level, coins, daily, work, transfer, buy limit.
- Mini games: flag, cartoon, chemistry, image guessing, math.

## Project Structure
```text
cmd/bot/                # entrypoint + core runtime logic
plugins/api/            # BotAPI contract + command context
plugins/general/        # ping, menu, runtime, profile, economy basics
plugins/owner/          # owner commands (prefix, owners, exec/eval, limits)
plugins/group/          # group features (antilink/welcome/goodbye/tagall)
plugins/media/          # media tools (sticker, qc, brat, upscale, etc.)
plugins/downloader/     # downloader commands (tiktok, instagram)
plugins/game/           # game commands
archive/music/          # archived music integration (mac-focused)
```

## Requirements
- Go `1.22+`
- MongoDB (Atlas `mongodb+srv://` recommended)
- `ffmpeg` (required for media conversion)
- `webpmux` (optional, for sticker watermark metadata)

Quick install on macOS:
```bash
brew install go ffmpeg webp
```

## Configuration (`config.json`)
Minimal example:
```json
{
  "mongo_uri": "mongodb+srv://USER:PASS@cluster.mongodb.net/?retryWrites=true&w=majority",
  "mongo_db": "meow_bot",
  "owner_number": ["6285171226069"],
  "bot_number": "17789019991"
}
```

Notes:
- `owner_number`: array of super-owner phone numbers (full access, including `$` and `x`).
- `bot_number`: default pairing number (optional).

## Run
Install dependencies:
```bash
go mod tidy
```

Run in interactive mode (recommended):
```bash
go run ./cmd/bot
```

Run with explicit flags (optional):
```bash
go run ./cmd/bot --auth pair --pair-phone 17789019991
```

## Command Overview
> Command prefix depends on active config (examples below use `.`).

General:
- `.ping`, `.runtime`, `.menu`, `.help`, `.prefix`
- `.profile`, `.leaderboard`, `.daily`, `.balance`, `.work`
- `.transfer <number> <coins>`, `.buylimit <amount>`
- `.ticket open <message>`, `.ticket my`, `.ticket info <id>`

Owner:
- `.addowner <number>`, `.delowner <number>`, `.listowner`
- `.setprefix .,!,#`, `.autoread on|off`, `.setwm pack|author`
- `.addlimit <number> <amount>`, `.resetlimit <number>`, `.dellimit <number>`
- `.setdaily <xp> <limit> <cooldown_hours>`
- `$ <shell command>` (super owner only)
- `x <expression>` (super owner only)

Group:
- `.antilink on|off`, `.welcome on|off`, `.goodbye on|off`
- `.setwelcome <template|reset>`, `.setgoodbye <template|reset>`
- `.tagall [message]`, `.hidetag [message]`

Media:
- `.sticker` (reply to image/video)
- `.toimg` (reply to sticker/image)
- `.toaudio`, `.tovn` (reply to video/audio)
- `.qc [text]`, `.brat <text>`, `.brat -animate <text>`
- `.upscale [2|4]` or `.hd`

Downloader:
- `.tiktok <url>`, `.tt <url>`
- `.instagram <url>`, `.ig <url>`

Game:
- `.tb`, `.tk`, `.tkimia`, `.tg`, `.math`

## Important Notes
- Do not commit sensitive runtime files (`config.json`, `session.db`).
- If MongoDB times out, verify URI, Atlas IP whitelist, and DNS/network.
- If sticker conversion fails, verify `ffmpeg` availability and webp support.

## Development
Format + build:
```bash
gofmt -w .
go build ./...
```

Run tests:
```bash
go test ./...
```

## License
This project is licensed under the MIT License. See [`LICENSE`](LICENSE).
