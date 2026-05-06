# syntax=docker/dockerfile:1

FROM golang:1.24-bookworm AS builder

WORKDIR /src

RUN apt-get update && apt-get install -y --no-install-recommends \
		gcc \
		libc6-dev \
		pkg-config \
	&& rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o /out/crazyzbot ./cmd

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
		ca-certificates \
		ffmpeg \
		imagemagick \
	&& rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/crazyzbot /app/crazyzbot
COPY .env.example /app/.env.example

RUN mkdir -p /app/data/tmp \
	&& printf '%s\n' \
		'#!/bin/sh' \
		'set -eu' \
		'subcommand="${1:-}"' \
		'if [ -z "$subcommand" ]; then' \
		'  exec /usr/bin/convert' \
		'fi' \
		'shift' \
		'case "$subcommand" in' \
		'  convert) exec /usr/bin/convert "$@" ;;' \
		'  identify) exec /usr/bin/identify "$@" ;;' \
		'  *) exec /usr/bin/convert "$subcommand" "$@" ;;' \
		'esac' \
	> /usr/local/bin/magick \
	&& chmod +x /usr/local/bin/magick

ENV SESSION_FILE=/app/data/session.db
ENV LOG_LEVEL=warn

CMD ["/app/crazyzbot"]
