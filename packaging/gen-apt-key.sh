#!/usr/bin/env bash
set -euo pipefail

# Gera o par de chaves GPG usado para assinar o repositorio APT do godesk.
#
# RODE ESTE SCRIPT NA SUA MAQUINA, NUNCA dentro de um assistente/CI/sandbox
# compartilhado — a chave privada nunca deve ser colada em chat, log ou
# issue. Este script so exporta:
#   - a chave PUBLICA em packaging/KEY.gpg (esse arquivo e' commitado, e'
#     seguro publicar)
#   - a chave PRIVADA (base64) e a passphrase em arquivos locais, ambos
#     ja adicionados ao .gitignore do projeto — copie o conteudo deles
#     para os secrets do GitHub Actions e depois apague os arquivos.
#
# Depois de rodar, va em:
#   GitHub -> Settings -> Secrets and variables -> Actions -> New repository secret
# e crie:
#   APT_GPG_PRIVATE_KEY   (conteudo de packaging/.secrets/private-key.b64)
#   APT_GPG_PASSPHRASE    (conteudo de packaging/.secrets/passphrase.txt)

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SECRETS_DIR="$PROJECT_ROOT/packaging/.secrets"
KEY_NAME="${GODESK_APT_KEY_NAME:-goDesk APT Repository}"
KEY_EMAIL="${GODESK_APT_KEY_EMAIL:-nathanq_@hotmail.com}"

mkdir -p "$SECRETS_DIR"

read -r -s -p "Defina uma passphrase para a chave GPG: " PASSPHRASE
echo
if [ -z "$PASSPHRASE" ]; then
  echo "ERRO: passphrase vazia" >&2
  exit 1
fi

echo "==> gerando par de chaves GPG (RSA 4096, sem expiracao)..."
gpg --batch --pinentry-mode loopback --passphrase "$PASSPHRASE" --quick-generate-key \
  "$KEY_NAME <$KEY_EMAIL>" rsa4096 sign never

KEY_ID="$(gpg --list-secret-keys --with-colons "$KEY_EMAIL" | awk -F: '/^sec/ {print $5; exit}')"
if [ -z "$KEY_ID" ]; then
  echo "ERRO: nao encontrei a chave recem-criada" >&2
  exit 1
fi

echo "==> exportando chave publica -> packaging/KEY.gpg (pode commitar)"
gpg --armor --export "$KEY_ID" > "$PROJECT_ROOT/packaging/KEY.gpg"

echo "==> exportando chave privada (NAO commitar) -> packaging/.secrets/"
gpg --batch --pinentry-mode loopback --passphrase "$PASSPHRASE" --armor --export-secret-keys "$KEY_ID" \
  | base64 -w0 > "$SECRETS_DIR/private-key.b64"
printf '%s' "$PASSPHRASE" > "$SECRETS_DIR/passphrase.txt"

cat <<EOF

ok: chave gerada (ID: $KEY_ID)

Proximos passos (manuais, na UI do GitHub):
  1. Settings -> Secrets and variables -> Actions -> New repository secret
       APT_GPG_PRIVATE_KEY = conteudo de $SECRETS_DIR/private-key.b64
       APT_GPG_PASSPHRASE  = conteudo de $SECRETS_DIR/passphrase.txt
  2. Depois de cadastrar os secrets, apague a pasta local:
       rm -rf $SECRETS_DIR
  3. Commite packaging/KEY.gpg (chave publica) normalmente.
EOF
