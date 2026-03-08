#!/usr/bin/env bash
set -euo pipefail

cd /home/container

if [[ ! -x ./meow-bot ]]; then
	cp /usr/local/bin/meow-bot ./meow-bot
	chmod +x ./meow-bot
fi

clear || true
echo "======================================"
echo "     Meow Bot Whatsmeow Devbox"
echo "======================================"
echo "User      : $(whoami)"
echo "Workdir   : $(pwd)"
echo "Bot       : $(./meow-bot --help >/dev/null 2>&1 && echo ready || echo ready)"
if command -v go >/dev/null 2>&1; then
	echo "Go        : $(go version)"
else
	echo "Go        : not installed (runtime image)"
fi
echo "Git       : $(git --version)"
echo "FFmpeg    : $(ffmpeg -version | head -n1)"
echo "SQLite    : $(sqlite3 --version | awk '{print $1}')"
echo "Timezone  : $(cat /etc/timezone 2>/dev/null || echo "${TZ:-unknown}")"
echo "======================================"
echo ""

if [[ ! -f ./config.json ]]; then
	echo "[entrypoint] warning: config.json belum ada di /home/container"
fi

if [[ "$#" -gt 0 ]]; then
	exec "$@"
fi

if [[ -n "${STARTUP:-}" ]]; then
	echo "[entrypoint] startup command: ${STARTUP}"
	exec /bin/bash -lc "${STARTUP}"
fi

exec ./meow-bot
