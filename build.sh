#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WEB_DIR="$ROOT/web"
OUTPUT="${1:-$ROOT/ufshare}"

# 颜色
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

info()  { echo -e "${GREEN}[build]${NC} $*"; }
warn()  { echo -e "${YELLOW}[build]${NC} $*"; }
error() { echo -e "${RED}[build]${NC} $*" >&2; exit 1; }

# 检查依赖
command -v node >/dev/null 2>&1 || error "node not found"
command -v npm  >/dev/null 2>&1 || error "npm not found"
command -v go   >/dev/null 2>&1 || error "go not found"

info "Building frontend..."
cd "$WEB_DIR"
npm install --silent
npm run build

info "Building Go binary -> $OUTPUT"
cd "$ROOT"
go build -trimpath -ldflags="-s -w" -o "$OUTPUT" ./cmd/ufshare

info "Done: $(du -sh "$OUTPUT" | cut -f1)  $OUTPUT"
