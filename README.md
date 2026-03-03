# bot-wa-go

Bot WhatsApp berbasis Go menggunakan [`whatsmeow`](https://github.com/tulir/whatsmeow), dengan login QR/pairing, sistem plugin, MongoDB storage, fitur media/downloader/game, dan economy system (XP, level, coins, limit).

## Fitur Utama
- Login interaktif: cukup `go run ./cmd/bot`, lalu pilih mode `QR` atau `Pairing` di terminal.
- Prefix multi dan bisa diubah owner: default `.`, `!`, `/`, `?`, `#`.
- Arsitektur plugin per kategori: `owner`, `group`, `media`, `downloader`, `game`, `general`.
- Penyimpanan data bot di MongoDB:
  - `settings`, `groups`, `users`, `tickets`.
- Sistem owner bertingkat:
  - `owner_number` di `config.json` = super owner.
  - owner tambahan via command (`addowner`).
- Fitur media: `sticker`, `toimg`, `toaudio`, `tovn`, `qc`, `brat`, `upscale`.
- Downloader: TikTok + Instagram.
- Group tools: `antilink`, `welcome`, `goodbye`, `tagall`, `hidetag`.
- Economy: profile, level/xp, coins, daily, work, transfer, buylimit.
- Game: tebak bendera, kartun, kimia, gambar, math.

## Struktur Project
```text
cmd/bot/                # entrypoint + core logic
plugins/api/            # kontrak BotAPI + Context
plugins/general/        # ping, menu, runtime, profile, economy umum
plugins/owner/          # owner tools (setprefix, owner mgmt, exec/eval, dll)
plugins/group/          # fitur group (antilink/welcome/goodbye/tagall)
plugins/media/          # media tools (sticker, qc, brat, upscale, dll)
plugins/downloader/     # downloader (tiktok, instagram)
plugins/game/           # game commands
archive/music/          # fitur music arsip (mac-focused)
```

## Requirements
- Go `1.22+`
- MongoDB (disarankan Atlas `mongodb+srv://...`)
- `ffmpeg` (wajib untuk media conversion)
- `webpmux` (opsional, untuk metadata watermark sticker)

MacOS install cepat:
```bash
brew install go ffmpeg webp
```

## Konfigurasi (`config.json`)
Contoh minimal:
```json
{
  "mongo_uri": "mongodb+srv://USER:PASS@cluster.mongodb.net/?retryWrites=true&w=majority",
  "mongo_db": "meow_bot",
  "owner_number": ["6285171226069"],
  "bot_number": "17789019991"
}
```

Keterangan:
- `owner_number`: array nomor super owner (akses penuh, termasuk `$` dan `x`).
- `bot_number`: nomor bot default untuk pairing (opsional).

## Menjalankan Bot
Install depedensi:
```bash
go mod tidy
```

Run interaktif (disarankan):
```bash
go run ./cmd/bot
```

Run non-interaktif (opsional):
```bash
go run ./cmd/bot --auth pair --pair-phone 17789019991
```

## Command Ringkas
> Prefix mengikuti setting aktif (contoh di bawah pakai `.`)

General:
- `.ping`, `.runtime`, `.menu`, `.help`, `.prefix`
- `.profile`, `.leaderboard`, `.daily`, `.balance`, `.work`
- `.transfer <nomor> <coins>`, `.buylimit <jumlah>`
- `.ticket open <pesan>`, `.ticket my`, `.ticket info <id>`

Owner:
- `.addowner <nomor>`, `.delowner <nomor>`, `.listowner`
- `.setprefix .,!,#`, `.autoread on|off`, `.setwm pack|author`
- `.addlimit <nomor> <jumlah>`, `.resetlimit <nomor>`, `.dellimit <nomor>`
- `.setdaily <xp> <limit> <cooldown_jam>`
- `$ <shell command>` (super owner only)
- `x <expression>` (super owner only)

Group:
- `.antilink on|off`, `.welcome on|off`, `.goodbye on|off`
- `.setwelcome <template|reset>`, `.setgoodbye <template|reset>`
- `.tagall [pesan]`, `.hidetag [pesan]`

Media:
- `.sticker` (reply image/video)
- `.toimg` (reply sticker/image)
- `.toaudio`, `.tovn` (reply video/audio)
- `.qc [text]`, `.brat <text>`, `.brat -animate <text>`
- `.upscale [2|4]` atau `.hd`

Downloader:
- `.tiktok <url>`, `.tt <url>`
- `.instagram <url>`, `.ig <url>`

Game:
- `.tb`, `.tk`, `.tkimia`, `.tg`, `.math`

## Notes Penting
- Jangan commit file sensitif (`config.json`, `session.db`).
- Jika Mongo timeout, cek URI, whitelist IP Atlas, dan DNS.
- Jika sticker gagal, cek binary `ffmpeg` dan dukungan webp encoder.

## Development
Format + build:
```bash
gofmt -w .
go build ./...
```

Test:
```bash
go test ./...
```

## License
Gunakan sesuai kebutuhan project kamu. Tambahkan lisensi resmi jika repo akan dibuka publik.
