#!/bin/sh
# usage: packaging/build-deb.sh BINARY VERSION REPO OUTDIR
set -eu
BIN=$1; V=${2#v}; REPO=$3; OUT=$4
ROOT=$(mktemp -d)
chmod 755 "$ROOT"
trap 'rm -rf "$ROOT"' EXIT
cp -r packaging/deb/DEBIAN "$ROOT/DEBIAN"
install -Dm755 "$BIN" "$ROOT/usr/bin/screensharex"
install -Dm644 packaging/default "$ROOT/etc/default/screensharex"
install -Dm644 -t "$ROOT/lib/systemd/system" packaging/systemd/*
install -d "$ROOT/usr/lib/screensharex"
sed "s|__REPO__|$REPO|" packaging/update.sh > "$ROOT/usr/lib/screensharex/update.sh"
chmod 755 "$ROOT/usr/lib/screensharex/update.sh" "$ROOT/DEBIAN/postinst" "$ROOT/DEBIAN/prerm"
cat > "$ROOT/DEBIAN/control" <<CTRL
Package: screensharex
Version: $V
Architecture: amd64
Maintainer: midnights-ra1n <noe@ngrt.fr>
Depends: adduser, curl, ffmpeg
Section: web
Priority: optional
Homepage: https://github.com/$REPO
Description: Self-hosted screenshot, GIF and video host
 Imgur/XBackBone-like media host with ShareX, XerahS, ishare,
 ScreenCloud, Spectacle and CLI uploaders.
CTRL
mkdir -p "$OUT"
dpkg-deb --root-owner-group --build "$ROOT" "$OUT/screensharex_${V}_amd64.deb"
