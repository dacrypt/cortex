#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
BIN_PATH="$BACKEND_DIR/cortexd"
AIR_BIN="${AIR_BIN:-air}"
CONFIG_SRC="$BACKEND_DIR/cortexd.yaml"
CONFIG_DST="$BACKEND_DIR/cortexd.local.yaml"

log() { printf '%s\n' "$*"; }
err() { printf 'Error: %s\n' "$*" >&2; }

if [[ ! -d "$BACKEND_DIR" ]]; then
  err "Backend dir not found: $BACKEND_DIR"
  exit 1
fi

if [[ "${SKIP_OLLAMA_CHECK:-0}" != "1" ]]; then
  if ! command -v ollama >/dev/null 2>&1; then
    err "Ollama is not installed. See LLM_SETUP.md for install steps."
    exit 1
  fi
  if ! ollama list >/dev/null 2>&1; then
    err "Ollama is not running. Start it with: ollama serve"
    exit 1
  fi
else
  log "Skipping Ollama check (SKIP_OLLAMA_CHECK=1)."
fi

if [[ ! -f "$CONFIG_DST" ]]; then
  if [[ -f "$CONFIG_SRC" ]]; then
    log "Creating local config: $CONFIG_DST"
    cp "$CONFIG_SRC" "$CONFIG_DST"
  else
    err "Config source not found: $CONFIG_SRC"
    exit 1
  fi
fi

if [[ "${SKIP_AIR:-0}" != "1" ]]; then
  if ! command -v "$AIR_BIN" >/dev/null 2>&1; then
    if command -v go >/dev/null 2>&1; then
      log "Air not found; installing..."
      go install github.com/air-verse/air@latest
    else
      err "Air not found and Go is missing; install Air or Go, or set SKIP_AIR=1."
      exit 1
    fi
  fi
  if command -v "$AIR_BIN" >/dev/null 2>&1; then
    log "Starting backend with Air (auto-reload)..."
    log "Frontend: run it from VS Code (F5) in a separate process."
    cd "$BACKEND_DIR"
    exec "$AIR_BIN" -c .air.toml
  else
    err "Air install completed but binary not in PATH; add GOPATH/bin to PATH."
    exit 1
  fi
fi

if [[ "${SKIP_BUILD:-0}" != "1" ]]; then
  if command -v go >/dev/null 2>&1; then
    log "Building backend..."
    (cd "$BACKEND_DIR" && go build -o cortexd ./cmd/cortexd)
  else
    if [[ -x "$BIN_PATH" ]]; then
      log "Go not found; using existing binary: $BIN_PATH"
    else
      err "Go is required to build the backend (or provide $BIN_PATH)."
      exit 1
    fi
  fi
else
  log "Skipping build (SKIP_BUILD=1)."
fi

log "Starting backend..."
log "Frontend: run it from VS Code (F5) in a separate process."
cd "$BACKEND_DIR"
exec ./cortexd --config cortexd.local.yaml
