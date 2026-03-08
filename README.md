# Meow Bot Docker Image (GHCR + Pterodactyl)

README ini diprioritaskan untuk container package GHCR agar langsung jelas cara pakai image Docker bot ini.

## Image
- Runtime (ringan): `ghcr.io/miftahganzz/bot-wa-go:latest`
- Devbox (lengkap Go): `ghcr.io/miftahganzz/bot-wa-go:dev-latest`

## Tujuan
Image ini untuk menjalankan bot Whatsmeow (`meow-bot`) di:
- Pterodactyl Panel
- Docker lokal / VPS

## Isi image
- Binary bot: `meow-bot`
- Media tools: `ffmpeg`, `webp`, `sqlite3`
- Terminal tools: `zsh`, `tmux`, `nano`, `vim-tiny`, `less`, `curl`, `wget`, `jq`, `git`
- Entrypoint: `docker/entrypoint.sh` (banner devbox + support env `STARTUP`)

## Perbedaan tag
- `latest`:
  - lebih kecil
  - fokus runtime
  - tidak include Go toolchain
- `dev-latest`:
  - include Go toolchain (`go version` tersedia)
  - cocok untuk debug/build di container
  - ukuran lebih besar

## Jalankan lokal
```bash
docker run --rm -it \
  -e STARTUP="./meow-bot --auth qr" \
  -v $(pwd)/config.json:/home/container/config.json \
  -v $(pwd)/session.db:/home/container/session.db \
  ghcr.io/miftahganzz/bot-wa-go:latest
```

## Setup Pterodactyl
- Docker image:
  - `ghcr.io/miftahganzz/bot-wa-go:latest`
  - atau `ghcr.io/miftahganzz/bot-wa-go:dev-latest`
- Startup command di egg: `{{STARTUP}}`
- Env `STARTUP` contoh:
  - `./meow-bot`
  - `./meow-bot --auth qr`
  - `./meow-bot --auth pair --pair-phone 62812xxxxxx --owner 62812xxxxxx`
- Simpan data runtime:
  - `/home/container/config.json`
  - `/home/container/session.db`

## Egg Pterodactyl
- `docker/pterodactyl/egg-meow-bot.json`
- `docker/pterodactyl/README.md`

## Sumber Docker config
Semua file Docker ada di folder:
- `docker/Dockerfile`
- `docker/Dockerfile.dev`
- `docker/entrypoint.sh`
- `docker/README.md`

## License
MIT. Lihat `LICENSE`.
