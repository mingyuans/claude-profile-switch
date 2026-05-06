#!/usr/bin/env bash
# ccs installer.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/mingyuans/claude-profile-switch/main/install.sh | bash
#   curl -fsSL .../install.sh | bash -s -- --version v0.1.0 --bin-dir ~/.local/bin
#
# Environment overrides (equivalent to flags):
#   CCS_REPO     repo slug,  default mingyuans/claude-profile-switch
#   CCS_VERSION  release tag, default latest
#   CCS_BIN_DIR  install dir, default /usr/local/bin (falls back to ~/.local/bin)
set -euo pipefail

REPO="${CCS_REPO:-mingyuans/claude-profile-switch}"
VERSION="${CCS_VERSION:-latest}"
BIN_DIR_OVERRIDE="${CCS_BIN_DIR:-}"

usage() {
  cat <<'EOF'
ccs installer

Usage:
  curl -fsSL https://raw.githubusercontent.com/mingyuans/claude-profile-switch/main/install.sh | bash
  bash install.sh [--version <tag>] [--bin-dir <path>] [--repo <owner/name>]

Flags / env vars:
  --version, CCS_VERSION   release tag (default: latest)
  --bin-dir, CCS_BIN_DIR   install dir  (default: /usr/local/bin, fallback ~/.local/bin)
  --repo,    CCS_REPO      repo slug   (default: mingyuans/claude-profile-switch)
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)  VERSION="$2"; shift 2 ;;
    --bin-dir)  BIN_DIR_OVERRIDE="$2"; shift 2 ;;
    --repo)     REPO="$2"; shift 2 ;;
    -h|--help)  usage; exit 0 ;;
    *) echo "unknown flag: $1" >&2; usage >&2; exit 1 ;;
  esac
done

err() { echo "  ✗ $*" >&2; exit 1; }
log() { echo "  ▸ $*"; }
ok()  { echo "  ✓ $*"; }

require() { command -v "$1" >/dev/null 2>&1 || err "missing required command: $1"; }

require curl
require tar
require uname

detect_platform() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64)  arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *) err "unsupported arch: $arch" ;;
  esac
  case "$os" in
    darwin|linux) ;;
    *) err "unsupported OS: $os" ;;
  esac
  printf '%s-%s\n' "$os" "$arch"
}

resolve_version() {
  if [[ "$VERSION" != "latest" ]]; then
    printf '%s\n' "$VERSION"
    return
  fi
  local tag
  tag="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null \
    | grep -E '"tag_name"' | head -1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
  [[ -n "$tag" ]] || err "could not resolve latest release for $REPO; pass --version explicitly"
  printf '%s\n' "$tag"
}

target_bin_dir() {
  if [[ -n "$BIN_DIR_OVERRIDE" ]]; then
    printf '%s\n' "$BIN_DIR_OVERRIDE"
    return
  fi
  if [[ -w /usr/local/bin ]] || { [[ ! -e /usr/local/bin ]] && [[ -w /usr/local ]]; }; then
    printf '%s\n' /usr/local/bin
    return
  fi
  printf '%s\n' "$HOME/.local/bin"
}

main() {
  local platform version bin_dir asset url tmp
  platform="$(detect_platform)"
  version="$(resolve_version)"
  bin_dir="$(target_bin_dir)"
  asset="ccs-${platform}.tar.gz"
  url="https://github.com/$REPO/releases/download/$version/$asset"

  log "platform: $platform"
  log "version:  $version"
  log "bin dir:  $bin_dir"

  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT

  log "downloading $url"
  curl -fsSL "$url" -o "$tmp/$asset" || err "download failed: $url"
  tar -xzf "$tmp/$asset" -C "$tmp"
  [[ -f "$tmp/ccs" ]] || err "archive did not contain a 'ccs' binary"
  chmod +x "$tmp/ccs"

  mkdir -p "$bin_dir" 2>/dev/null || true
  if ! install -m 0755 "$tmp/ccs" "$bin_dir/ccs" 2>/dev/null; then
    err "could not write to $bin_dir; rerun with sudo or pass --bin-dir <path>"
  fi
  ok "installed to $bin_dir/ccs"
  ok "version: $("$bin_dir/ccs" version 2>/dev/null || echo unknown)"

  case ":$PATH:" in
    *":$bin_dir:"*) ;;
    *) log "note: $bin_dir is not on your PATH; add it via your shell rc" ;;
  esac

  echo
  log "to make 'ccs use' take effect in the current shell, install the integration once:"
  echo "      echo 'eval \"\$(ccs init zsh)\"' >> ~/.zshrc && exec zsh"
}

main "$@"
