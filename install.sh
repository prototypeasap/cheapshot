#!/bin/sh
set -e

REPO="prototypeasap/cheapshot"
INSTALL_DIR="${CHEAPSHOT_INSTALL_DIR:-/usr/local/bin}"

get_latest_version() {
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4
}

detect_platform() {
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  arch=$(uname -m)

  case "$os" in
    darwin) os="darwin" ;;
    linux)  os="linux" ;;
    mingw*|msys*|cygwin*) os="windows" ;;
    *) echo "Unsupported OS: $os" >&2; exit 1 ;;
  esac

  case "$arch" in
    x86_64|amd64)  arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) echo "Unsupported architecture: $arch" >&2; exit 1 ;;
  esac

  echo "${os}_${arch}"
}

main() {
  platform=$(detect_platform)
  version=$(get_latest_version)

  if [ -z "$version" ]; then
    echo "Failed to determine latest version" >&2
    exit 1
  fi

  if echo "$platform" | grep -q "windows"; then
    ext="zip"
  else
    ext="tar.gz"
  fi

  url="https://github.com/${REPO}/releases/download/${version}/cheapshot_${platform}.${ext}"
  echo "Installing cheapshot ${version} (${platform})..."

  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' EXIT

  curl -fsSL "$url" -o "${tmpdir}/cheapshot.${ext}"

  if [ "$ext" = "zip" ]; then
    unzip -q "${tmpdir}/cheapshot.${ext}" -d "$tmpdir"
  else
    tar -xzf "${tmpdir}/cheapshot.${ext}" -C "$tmpdir"
  fi

  if [ ! -w "$INSTALL_DIR" ]; then
    echo "Installing to ${INSTALL_DIR} (requires sudo)..."
    sudo install -m 755 "${tmpdir}/cheapshot" "$INSTALL_DIR/cheapshot"
  else
    install -m 755 "${tmpdir}/cheapshot" "$INSTALL_DIR/cheapshot"
  fi

  echo "cheapshot ${version} installed to ${INSTALL_DIR}/cheapshot"
}

main
