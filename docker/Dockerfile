# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.25

FROM golang:${GO_VERSION}-bookworm AS builder
WORKDIR /src

ENV CGO_ENABLED=1 GO111MODULE=on

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
	--mount=type=cache,target=/go/pkg/mod \
	go build -trimpath -ldflags="-s -w" -o /out/meow-bot ./cmd/bot

FROM debian:bookworm-slim
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
	ca-certificates \
	tzdata \
	ffmpeg \
	webp \
	bash \
	zsh \
	tmux \
	nano \
	vim-tiny \
	less \
	curl \
	wget \
	jq \
	git \
	procps \
	iputils-ping \
	sqlite3 \
	&& rm -rf /var/lib/apt/lists/*

RUN useradd -m -d /home/container -s /usr/bin/zsh container \
	&& printf '%s\n' \
		'export TERM=xterm-256color' \
		'PROMPT="%F{cyan}%n@%m%f:%F{green}%~%f %# "' \
		'alias ll="ls -alF --color=auto"' \
		'alias la="ls -A --color=auto"' \
		'alias l="ls -CF --color=auto"' \
	> /home/container/.zshrc \
	&& chown container:container /home/container/.zshrc

WORKDIR /home/container

COPY --from=builder /out/meow-bot /usr/local/bin/meow-bot
COPY docker/entrypoint.sh /entrypoint.sh

RUN chmod +x /entrypoint.sh && chown -R container:container /home/container

USER container

ENV TZ=Asia/Jakarta
ENV SHELL=/usr/bin/zsh
ENV STARTUP="./meow-bot"

ENTRYPOINT ["/entrypoint.sh"]
