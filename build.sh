#!/usr/bin/env bash
set -euo pipefail

OS="${GOOS:-linux}"
ARCH="${GOARCH:-amd64}"
OUTPUT="ufshare"

while [[ $# -gt 0 ]]; do
	case "$1" in
	--os)
		OS="$2"
		shift 2
		;;
	--arch)
		ARCH="$2"
		shift 2
		;;
	--output)
		OUTPUT="$2"
		shift 2
		;;
	*)
		echo "Usage: $0 [--os windows|darwin|linux] [--arch amd64|arm64] [--output <name>]"
		exit 1
		;;
	esac
done

ROOT="$(cd "$(dirname "$0")" && pwd)"

echo "==> Building frontend..."
cd "$ROOT/frontend"
npm install --silent
npm run build
cd "$ROOT"

echo "==> Building binary..."

GOOS="$OS"
GOARCH="$ARCH"
CGO_ENABLED=0
BIN="$OUTPUT"
if [ "$GOOS" = "windows" ]; then
	BIN="${BIN}.exe"
fi

export GOOS GOARCH CGO_ENABLED
echo "    GOOS=$GOOS  GOARCH=$GOARCH  ->  $BIN"
go build -ldflags="-s -w" -o "$BIN" .
echo "==> Built: $BIN"
