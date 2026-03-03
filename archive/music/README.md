# Archived Music Commands

Semua fitur command music (macOS-only) sudah dipisahkan dari bot aktif dan di-archive di folder ini.

Alasan:
- fitur khusus Mac, tidak universal
- biar command inti bot tetap ringan dan fokus

Catatan:
- command `.music ...` sudah dinonaktifkan dari plugin owner
- menu/help aktif sudah tidak menampilkan command music
- jika mau aktif lagi nanti, restore handler music ke `plugins/owner/` dan core `cmd/bot/`

Isi archive:
- `mac_music.go` : core backend music (status, list/search, queue, playlist, online search, preview play, now-playing helpers)
- `owner_music.go` : menu + wrapper style command owner
