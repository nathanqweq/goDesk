#!/usr/bin/env bash
set -euo pipefail

# Monta o pacote .deb do godesk-client (binario + env default + modulo
# frontend do Zabbix) — instalado no host que roda a UI do Zabbix.
# Uso: packaging/build-deb-client.sh VERSION
# Saida: build/godesk-client_<VERSION>_amd64.deb

if [ $# -lt 1 ]; then
  echo "uso: $0 VERSION" >&2
  exit 1
fi

VERSION="$1"
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BUILD_DIR="$PROJECT_ROOT/build"
PKG_ROOT="$BUILD_DIR/godesk-client_${VERSION}_amd64"

echo "==> limpando build anterior"
rm -rf "$PKG_ROOT"
mkdir -p "$BUILD_DIR"

echo "==> compilando binario (linux/amd64)"
mkdir -p "$PKG_ROOT/usr/bin"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -C "$PROJECT_ROOT" \
  -o "$PKG_ROOT/usr/bin/godesk-client" \
  ./cmd/godesk-client
chmod 0755 "$PKG_ROOT/usr/bin/godesk-client"

echo "==> config default (conffile)"
mkdir -p "$PKG_ROOT/etc/zabbix/godesk"
cp "$PROJECT_ROOT/dist/godesk-client.env" "$PKG_ROOT/etc/zabbix/godesk/godesk-client.env"

echo "==> modulo frontend do Zabbix"
MODULE_DEST="$PKG_ROOT/usr/share/zabbix/ui/modules/goDesk"
mkdir -p "$(dirname "$MODULE_DEST")"
cp -r "$PROJECT_ROOT/Module/goDesk" "$MODULE_DEST"
find "$MODULE_DEST" -type d -exec chmod 755 {} \;
find "$MODULE_DEST" -type f -exec chmod 644 {} \;

echo "==> metadados DEBIAN"
mkdir -p "$PKG_ROOT/DEBIAN"
sed "s/__VERSION__/${VERSION}/" "$PROJECT_ROOT/packaging/debian-client/control.template" > "$PKG_ROOT/DEBIAN/control"
cp "$PROJECT_ROOT/packaging/debian-client/conffiles" "$PKG_ROOT/DEBIAN/conffiles"
cp "$PROJECT_ROOT/packaging/debian-client/postinst" "$PKG_ROOT/DEBIAN/postinst"
chmod 0755 "$PKG_ROOT/DEBIAN/postinst"

echo "==> gerando .deb"
dpkg-deb --build --root-owner-group "$PKG_ROOT" "$BUILD_DIR/godesk-client_${VERSION}_amd64.deb"

echo "ok: $BUILD_DIR/godesk-client_${VERSION}_amd64.deb"
