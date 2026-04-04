#!/bin/sh
set -e

REPO="cloudticon/ct"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="ct"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

SUFFIX=""
case "$OS" in
  linux|darwin) ;;
  mingw*|msys*|cygwin*)
    OS="windows"
    SUFFIX=".exe"
    ;;
  *)
    echo "Unsupported OS: $OS" >&2
    exit 1
    ;;
esac

ASSET_NAME="${BINARY_NAME}-${OS}-${ARCH}${SUFFIX}"

echo "Detecting system: ${OS}/${ARCH}"
echo "Fetching latest release from github.com/${REPO}..."

LATEST_URL="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep "browser_download_url.*${ASSET_NAME}" \
  | head -1 \
  | cut -d '"' -f 4)"

if [ -z "$LATEST_URL" ]; then
  echo "Could not find asset ${ASSET_NAME} in latest release" >&2
  exit 1
fi

echo "Downloading ${LATEST_URL}..."
TMP="$(mktemp)"
curl -fsSL -o "$TMP" "$LATEST_URL"
chmod +x "$TMP"

echo "Installing to ${INSTALL_DIR}/${BINARY_NAME}${SUFFIX}..."
mv "$TMP" "${INSTALL_DIR}/${BINARY_NAME}${SUFFIX}"

echo "Done! Run 'ct --help' to get started."
