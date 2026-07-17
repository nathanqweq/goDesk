#!/usr/bin/env bash
set -euo pipefail

# Atualiza o repositorio APT (branch gh-pages) com um novo .deb: copia para
# o pool, regenera os indices (Packages/Release) e assina (InRelease /
# Release.gpg). Nao faz commit/push — isso fica a cargo do workflow.
#
# Variaveis obrigatorias:
#   DEB_FILE             caminho do .deb recem-buildado
#   GHPAGES_DIR           checkout local do branch gh-pages
#   APT_GPG_PRIVATE_KEY   chave privada GPG, armored + base64 (secret do CI)
#   APT_GPG_PASSPHRASE    passphrase da chave

: "${DEB_FILE:?defina DEB_FILE}"
: "${GHPAGES_DIR:?defina GHPAGES_DIR}"
: "${APT_GPG_PRIVATE_KEY:?defina APT_GPG_PRIVATE_KEY}"
: "${APT_GPG_PASSPHRASE:?defina APT_GPG_PASSPHRASE}"

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SUITE="stable"
POOL_DIR="$GHPAGES_DIR/pool/main/g/godesk"
DIST_DIR="$GHPAGES_DIR/dists/$SUITE"
BINARY_DIR="$DIST_DIR/main/binary-amd64"

echo "==> preparando keyring temporario"
export GNUPGHOME
GNUPGHOME="$(mktemp -d)"
trap 'rm -rf "$GNUPGHOME"' EXIT
chmod 700 "$GNUPGHOME"

echo "$APT_GPG_PRIVATE_KEY" | base64 -d | gpg --batch --import
KEY_ID="$(gpg --batch --list-secret-keys --with-colons | awk -F: '/^sec/ {print $5; exit}')"
if [ -z "$KEY_ID" ]; then
  echo "ERRO: falha ao importar a chave GPG" >&2
  exit 1
fi

echo "==> copiando .deb para o pool"
mkdir -p "$POOL_DIR" "$BINARY_DIR"
cp "$DEB_FILE" "$POOL_DIR/"

echo "==> regenerando indice Packages"
(cd "$GHPAGES_DIR" && dpkg-scanpackages pool /dev/null) > "$BINARY_DIR/Packages"
gzip -9 -k -f "$BINARY_DIR/Packages"

echo "==> gerando Release"
apt-ftparchive \
  -o APT::FTPArchive::Release::Origin="goDesk" \
  -o APT::FTPArchive::Release::Label="goDesk" \
  -o APT::FTPArchive::Release::Suite="$SUITE" \
  -o APT::FTPArchive::Release::Codename="$SUITE" \
  -o APT::FTPArchive::Release::Architectures="amd64" \
  -o APT::FTPArchive::Release::Components="main" \
  release "$DIST_DIR" > "$DIST_DIR/Release"

echo "==> assinando Release"
gpg --batch --yes --pinentry-mode loopback --passphrase "$APT_GPG_PASSPHRASE" \
  --local-user "$KEY_ID" --clearsign -o "$DIST_DIR/InRelease" "$DIST_DIR/Release"
gpg --batch --yes --pinentry-mode loopback --passphrase "$APT_GPG_PASSPHRASE" \
  --local-user "$KEY_ID" -abs -o "$DIST_DIR/Release.gpg" "$DIST_DIR/Release"

echo "==> publicando chave publica"
cp "$PROJECT_ROOT/packaging/KEY.gpg" "$GHPAGES_DIR/KEY.gpg"

echo "ok: repositorio APT atualizado em $GHPAGES_DIR"
