#!/usr/bin/env sh
# Install the latest `kc` (Kvindo Cloud CLI) for your OS/architecture.
#   curl -fsSL https://raw.githubusercontent.com/Kvindo/kc-cli/main/install.sh | sh
# Override the install directory with KC_INSTALL_DIR (default: /usr/local/bin).
set -eu

REPO="Kvindo/kc-cli"
INSTALL_DIR="${KC_INSTALL_DIR:-/usr/local/bin}"

# ── detect OS ──────────────────────────────────────────────────────────────
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in
  linux)  os="linux" ;;
  darwin) os="darwin" ;;
  *)
    echo "kc: unsupported OS \"$os\"." >&2
    echo "    On Windows, download kc-windows-amd64.exe from:" >&2
    echo "    https://github.com/$REPO/releases/latest" >&2
    exit 1 ;;
esac

# ── detect architecture ────────────────────────────────────────────────────
arch="$(uname -m)"
case "$arch" in
  x86_64|amd64)  arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "kc: unsupported architecture \"$arch\"." >&2; exit 1 ;;
esac

asset="kc-${os}-${arch}"
url="https://github.com/$REPO/releases/latest/download/$asset"

# ── download ───────────────────────────────────────────────────────────────
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
echo "Downloading $asset from the latest release..."
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$url" -o "$tmp"
elif command -v wget >/dev/null 2>&1; then
  wget -qO "$tmp" "$url"
else
  echo "kc: need curl or wget to download." >&2
  exit 1
fi
chmod +x "$tmp"

# ── install (use sudo only if the target dir isn't writable) ───────────────
dest="$INSTALL_DIR/kc"
if [ -w "$INSTALL_DIR" ]; then
  mv "$tmp" "$dest"
else
  echo "Installing to $dest (needs sudo)..."
  sudo mv "$tmp" "$dest"
fi
trap - EXIT

echo "Installed kc -> $dest"
"$dest" version 2>/dev/null || echo "Run 'kc help' to get started."
