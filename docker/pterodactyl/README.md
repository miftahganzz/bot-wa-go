# Pterodactyl Egg

File egg siap import:
- `egg-meow-bot.json`

## Cara pakai
1. Buka Panel Admin Pterodactyl.
2. Masuk ke `Nests` -> pilih/buat nest -> `Import Egg`.
3. Upload `egg-meow-bot.json`.
4. Setelah ter-import, edit field `docker_images` dan ganti:
   - `ghcr.io/<owner>/<repo>:latest`
   - `ghcr.io/<owner>/<repo>:dev-latest`
   dengan repository GHCR kamu.
5. Buat server baru dari egg ini.
6. Set startup env:
   - `STARTUP=./meow-bot`
   - atau `STARTUP=./meow-bot --auth qr`

## Catatan data penting
- Simpan `config.json` dan `session.db` di `/home/container`.
- Folder `/home/container` adalah persistent storage server Pterodactyl.
