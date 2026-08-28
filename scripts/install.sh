#!/bin/sh
#
# Installs the latest (or a pinned) cloudtui release binary for macOS/Linux.
# No sudo, no PATH/shell-rc edits — extracts into a per-user directory and
# prints a hint if that directory isn't already on PATH.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/ePex/cloudtui/main/scripts/install.sh | sh
#
# Env vars:
#   CLOUDTUI_VERSION     release tag to install, e.g. v0.3.0 (default: latest)
#   CLOUDTUI_INSTALL_DIR directory to install the binary into (default: $HOME/.local/bin)
#
# See scripts/install.ps1 for the Windows equivalent.

set -eu

REPO="ePex/cloudtui"
INSTALL_DIR="${CLOUDTUI_INSTALL_DIR:-$HOME/.local/bin}"

os="$(uname -s)"
case "$os" in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	*)
		echo "install.sh: unsupported OS: $os (Windows? use scripts/install.ps1)" >&2
		exit 1
		;;
esac

arch="$(uname -m)"
case "$arch" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*)
		echo "install.sh: unsupported architecture: $arch" >&2
		exit 1
		;;
esac

tag="${CLOUDTUI_VERSION:-}"
if [ -z "$tag" ]; then
	latest_url="$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest")"
	tag="${latest_url##*/}"
	if [ -z "$tag" ] || [ "$tag" = "latest" ]; then
		echo "install.sh: could not resolve the latest release tag" >&2
		exit 1
	fi
fi
version="${tag#v}"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

archive="cloudtui_${version}_${os}_${arch}.tar.gz"
base_url="https://github.com/$REPO/releases/download/$tag"

echo "install.sh: downloading $archive ($tag)..."
curl -fsSL -o "$tmpdir/$archive" "$base_url/$archive"
curl -fsSL -o "$tmpdir/checksums.txt" "$base_url/checksums.txt"

line="$(awk -v f="$archive" '$2 == f { print; exit }' "$tmpdir/checksums.txt")"
if [ -z "$line" ]; then
	echo "install.sh: no checksum entry for $archive in checksums.txt" >&2
	exit 1
fi
printf '%s\n' "$line" >"$tmpdir/checksum.txt"

if command -v sha256sum >/dev/null 2>&1; then
	(cd "$tmpdir" && sha256sum -c checksum.txt) || {
		echo "install.sh: checksum verification failed" >&2
		exit 1
	}
elif command -v shasum >/dev/null 2>&1; then
	(cd "$tmpdir" && shasum -a 256 -c checksum.txt) || {
		echo "install.sh: checksum verification failed" >&2
		exit 1
	}
else
	echo "install.sh: neither sha256sum nor shasum found; cannot verify checksum" >&2
	exit 1
fi

mkdir -p "$INSTALL_DIR"
tar -xzf "$tmpdir/$archive" -C "$tmpdir" cloudtui
mv "$tmpdir/cloudtui" "$INSTALL_DIR/cloudtui"
chmod +x "$INSTALL_DIR/cloudtui"

echo "install.sh: installed cloudtui $tag to $INSTALL_DIR/cloudtui"

case ":$PATH:" in
	*":$INSTALL_DIR:"*) ;;
	*)
		echo "install.sh: $INSTALL_DIR is not on your PATH."
		echo "  Add it, e.g.: export PATH=\"$INSTALL_DIR:\$PATH\""
		;;
esac
