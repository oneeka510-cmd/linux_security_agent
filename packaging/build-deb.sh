#!/bin/bash
# Builds dist/go-agent_<version>_amd64.deb
# Usage: bash packaging/build-deb.sh [version]      (run it inside WSL / Linux)
set -euo pipefail

VERSION="${1:-0.1.0}"
ARCH="amd64"
NAME="go-agent"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
STAGE="$(mktemp -d)"   # build the file tree on Linux's own disk so permissions are right
trap 'rm -rf "$STAGE"' EXIT
chmod 0755 "$STAGE"    # mktemp makes it private (0700); a package's top folder must be 0755

echo "Building $NAME $VERSION ($ARCH)"

# 1. The program: a single static binary, so it needs nothing else installed.
mkdir -p "$STAGE/usr/bin"
(cd "$ROOT/agent" && CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" \
    go build -trimpath -ldflags "-s -w -X main.agentVersion=$VERSION" -o "$STAGE/usr/bin/$NAME" .)

# 2. The config template (no secrets inside) and the systemd files.
mkdir -p "$STAGE/etc/$NAME" "$STAGE/lib/systemd/system"
cp "$ROOT/packaging/config.json" "$STAGE/etc/$NAME/config.json"
cp "$ROOT/packaging/go-agent.service" "$ROOT/packaging/go-agent.timer" "$STAGE/lib/systemd/system/"

# 3. The package's own description and install scripts.
mkdir -p "$STAGE/DEBIAN"
cat > "$STAGE/DEBIAN/control" <<EOF
Package: $NAME
Version: $VERSION
Section: admin
Priority: optional
Architecture: $ARCH
Maintainer: go-agent maintainers <root@localhost>
Description: Endpoint security agent
 Collects the installed packages and runs CIS Benchmark (Ubuntu 22.04, Level 1)
 configuration checks, then sends the results to a cloud API.
EOF
# Files listed here are treated as settings: an upgrade will not overwrite edits.
echo "/etc/$NAME/config.json" > "$STAGE/DEBIAN/conffiles"
cp "$ROOT/packaging/postinst" "$ROOT/packaging/prerm" "$ROOT/packaging/postrm" "$STAGE/DEBIAN/"

# 4. Permissions: program and scripts executable, config readable only by root.
chmod 0755 "$STAGE/usr/bin/$NAME" "$STAGE/DEBIAN/postinst" "$STAGE/DEBIAN/prerm" "$STAGE/DEBIAN/postrm"
chmod 0600 "$STAGE/etc/$NAME/config.json"
chmod 0644 "$STAGE/lib/systemd/system/go-agent.service" "$STAGE/lib/systemd/system/go-agent.timer"

# 5. Pack it.
mkdir -p "$ROOT/dist"
dpkg-deb --root-owner-group --build "$STAGE" "$ROOT/dist/${NAME}_${VERSION}_${ARCH}.deb"
