# Docker (Pterodactyl + GHCR)

Folder ini khusus untuk image Docker bot `meow`, supaya terpisah dari source utama.

## Isi
- `Dockerfile`: multi-stage build (build Go binary lalu runtime slim).
- `entrypoint.sh`: startup script kompatibel Pterodactyl (`STARTUP` env).

## Build image lokal
Jalankan dari root project:

```bash
docker build -f docker/Dockerfile -t meow-bot:latest .
```

## Jalankan lokal
```bash
docker run --rm -it \
  -e STARTUP="./meow-bot --auth qr" \
  -v $(pwd)/config.json:/home/container/config.json \
  -v $(pwd)/session.db:/home/container/session.db \
  meow-bot:latest
```

## Setup di Pterodactyl (custom image)
- Docker image:
  - GHCR ringan (recommended): `ghcr.io/<username>/<repo>:latest`
  - GHCR dev lengkap Go: `ghcr.io/<username>/<repo>:dev-latest`
- Startup command: `{{STARTUP}}`
- Environment `STARTUP` (contoh):
  - `./meow-bot`
  - `./meow-bot --auth qr`
  - `./meow-bot --auth pair --pair-phone 62812xxxxxx --owner 62812xxxxxx`
- Mount/volume penting:
  - `config.json` ke `/home/container/config.json`
  - `session.db` ke `/home/container/session.db`

## Kenapa image ini cocok
- Tetap relatif ringan: runtime pakai `debian:bookworm-slim`.
- Paket bot lengkap: `ffmpeg`, `webp`, `sqlite3`, `ca-certificates`, `tzdata`.
- Terminal nyaman untuk operasional: `zsh`, `tmux`, `nano`, `vim-tiny`, `less`, `curl`, `wget`, `jq`, `git`.

## Varian image
- `latest`:
  - ukuran lebih kecil
  - fokus runtime bot
  - tidak include Go toolchain
- `dev-latest`:
  - include Go toolchain (`go version` tersedia)
  - cocok untuk debug/live edit/build di container
  - ukuran lebih besar

## Publish ke GHCR (otomatis)
Workflow tersedia di `.github/workflows/ghcr.yml`.

Trigger:
- push ke branch `main`
- tag `v*`
- manual (`workflow_dispatch`)

Image yang dipublish:
- Branch `main`:
  - `ghcr.io/<owner>/<repo>:latest`
  - `ghcr.io/<owner>/<repo>:dev-latest`
- Tag release `vX.Y.Z`:
  - `ghcr.io/<owner>/<repo>:vX.Y.Z`
  - `ghcr.io/<owner>/<repo>:vX.Y.Z-dev`

Syarat:
- Repository ada di GitHub.
- Package permission `write` aktif untuk workflow.

## Egg Pterodactyl
Template egg siap import ada di:
- `docker/pterodactyl/egg-meow-bot.json`
- `docker/pterodactyl/README.md`
