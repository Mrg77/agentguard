#!/bin/sh
# agentguard installer — https://github.com/Mrg77/agentguard
# Works on Linux (Debian, Ubuntu, Alpine…) and macOS.
# Usage: curl -fsSL https://raw.githubusercontent.com/Mrg77/agentguard/main/install.sh | sh
set -eu

REPO="Mrg77/agentguard"
INSTALL_DIR="${AGENTGUARD_INSTALL_DIR:-$HOME/.local/bin}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  darwin|linux) ;;
  *) echo "error: unsupported OS '$os' (darwin and linux only — use WSL on Windows)" >&2; exit 1 ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "error: unsupported architecture '$arch'" >&2; exit 1 ;;
esac

version="${AGENTGUARD_VERSION:-}"
if [ -z "$version" ]; then
  version=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name"' | head -1 | cut -d '"' -f 4)
fi
[ -n "$version" ] || { echo "error: could not resolve latest release" >&2; exit 1; }

archive="agentguard_${version#v}_${os}_${arch}.tar.gz"
url="https://github.com/$REPO/releases/download/$version/$archive"

echo "Downloading agentguard $version ($os/$arch)..."
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
curl -fsSL "$url" -o "$tmp/$archive"
tar -xzf "$tmp/$archive" -C "$tmp" agentguard

mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/agentguard" "$INSTALL_DIR/agentguard"
echo "Installed to $INSTALL_DIR/agentguard"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "note: add $INSTALL_DIR to your PATH, e.g.:"
     echo "  echo 'export PATH=\"$INSTALL_DIR:\$PATH\"' >> ~/.bashrc" ;;
esac

echo
echo "Get started:"
echo "  agentguard init                          # write a starter policy"
echo "  agentguard policy test <tool> <target>   # check what a rule decides"
echo "  agentguard scan ~/.cursor/mcp.json       # audit an MCP config for risks"
echo "  agentguard interpose -- npx <mcp-server>  # firewall an MCP tool server"
