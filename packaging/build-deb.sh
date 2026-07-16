#!/usr/bin/env bash
set -euo pipefail

# Monta o pacote .deb do godesk (binario + servico systemd + configs default).
# Uso: packaging/build-deb.sh VERSION
# Saida: build/godesk_<VERSION>_amd64.deb

if [ $# -lt 1 ]; then
  echo "uso: $0 VERSION" >&2
  exit 1
fi

VERSION="$1"
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BUILD_DIR="$PROJECT_ROOT/build"
PKG_ROOT="$BUILD_DIR/godesk_${VERSION}_amd64"

echo "==> limpando build anterior"
rm -rf "$PKG_ROOT"
mkdir -p "$BUILD_DIR"

echo "==> compilando binario (linux/amd64)"
mkdir -p "$PKG_ROOT/usr/lib/zabbix/alertscripts"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -C "$PROJECT_ROOT" \
  -o "$PKG_ROOT/usr/lib/zabbix/alertscripts/godesk" \
  ./cmd/godesk
chmod 0755 "$PKG_ROOT/usr/lib/zabbix/alertscripts/godesk"

echo "==> symlink /usr/bin/godesk"
mkdir -p "$PKG_ROOT/usr/bin"
ln -sf /usr/lib/zabbix/alertscripts/godesk "$PKG_ROOT/usr/bin/godesk"

echo "==> unit systemd"
mkdir -p "$PKG_ROOT/usr/lib/systemd/system"
cp "$PROJECT_ROOT/dist/godesk.service" "$PKG_ROOT/usr/lib/systemd/system/godesk.service"

echo "==> configs default (conffiles)"
mkdir -p "$PKG_ROOT/etc/zabbix/godesk"
cp "$PROJECT_ROOT/dist/godesk-config.yaml" "$PKG_ROOT/etc/zabbix/godesk/godesk-config.yaml"
cp "$PROJECT_ROOT/dist/godesk-smtp-config.env" "$PKG_ROOT/etc/zabbix/godesk/godesk-smtp-config.env"
cp "$PROJECT_ROOT/dist/godesk-service.env" "$PKG_ROOT/etc/zabbix/godesk/godesk-service.env"

echo "==> metadados DEBIAN"
mkdir -p "$PKG_ROOT/DEBIAN"
sed "s/__VERSION__/${VERSION}/" "$PROJECT_ROOT/packaging/debian/control.template" > "$PKG_ROOT/DEBIAN/control"
cp "$PROJECT_ROOT/packaging/debian/conffiles" "$PKG_ROOT/DEBIAN/conffiles"
cp "$PROJECT_ROOT/packaging/debian/postinst" "$PKG_ROOT/DEBIAN/postinst"
cp "$PROJECT_ROOT/packaging/debian/postrm" "$PKG_ROOT/DEBIAN/postrm"
chmod 0755 "$PKG_ROOT/DEBIAN/postinst" "$PKG_ROOT/DEBIAN/postrm"

echo "==> gerando .deb"
dpkg-deb --build --root-owner-group "$PKG_ROOT" "$BUILD_DIR/godesk_${VERSION}_amd64.deb"

echo "ok: $BUILD_DIR/godesk_${VERSION}_amd64.deb"
