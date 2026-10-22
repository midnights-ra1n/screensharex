#!/bin/sh
# Runs as root (screensharex-update.service). The web UI only writes the wanted tag;
# the package is always downloaded from this project's GitHub releases and checksum-verified.
set -eu
REPO="__REPO__"
REQ=/var/lib/screensharex/update-request
TAG=$(head -c 32 "$REQ" | tr -d '\n')
rm -f "$REQ"
echo "$TAG" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' || { echo "invalid tag: $TAG" >&2; exit 1; }
V=${TAG#v}
DEB="screensharex_${V}_amd64.deb"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
curl -fsSL -o "$TMP/$DEB" "https://github.com/$REPO/releases/download/$TAG/$DEB"
curl -fsSL -o "$TMP/SHA256SUMS" "https://github.com/$REPO/releases/download/$TAG/SHA256SUMS"
(cd "$TMP" && grep " $DEB\$" SHA256SUMS | sha256sum -c -)
DEBIAN_FRONTEND=noninteractive dpkg -i "$TMP/$DEB"
