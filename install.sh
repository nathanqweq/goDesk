#!/usr/bin/env bash
set -euo pipefail

echo "=== goDesk module + binary installer ==="

# caminho do projeto (onde estao Module/goDesk e dist/)
PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"

# --- FRONTEND MODULE ---
SRC_MODULE="$PROJECT_ROOT/Module/goDesk"
DEST_MODULE="/usr/share/zabbix/ui/modules/goDesk"

# --- BINARY (Zabbix alertscripts) ---
SRC_DIST="$PROJECT_ROOT/dist"
DEST_BIN_DIR="/usr/lib/zabbix/alertscripts"
DEST_BIN="$DEST_BIN_DIR/godesk"

# --- CONFIG FILES ---
DEST_CFG_DIR="/etc/zabbix/godesk"
SRC_CFG_YAML="$SRC_DIST/godesk-config.yaml"
SRC_CFG_SMTP="$SRC_DIST/godesk-smtp-config.env"
DEST_CFG_YAML="$DEST_CFG_DIR/godesk-config.yaml"
DEST_CFG_SMTP="$DEST_CFG_DIR/godesk-smtp-config.env"

if [ "${EUID:-$(id -u)}" -ne 0 ]; then
  echo "rode como root: sudo $0"
  exit 1
fi

check_file() {
  if [ ! -f "$1" ]; then
    echo "ERRO: faltando arquivo $1"
    exit 1
  fi
}

check_dir() {
  if [ ! -d "$1" ]; then
    echo "ERRO: faltando diretorio $1"
    exit 1
  fi
}

validate_yaml_config() {
  local f="$1"
  check_file "$f"

  if [ ! -s "$f" ]; then
    echo "ERRO: YAML vazio: $f"
    exit 1
  fi

  grep -Eq '^default:' "$f" || { echo "ERRO: YAML invalido (faltando default:) em $f"; exit 1; }
  grep -Eq '^clients:' "$f" || { echo "ERRO: YAML invalido (faltando clients:) em $f"; exit 1; }
  grep -Eq '^[[:space:]]+topdesk:' "$f" || { echo "ERRO: YAML invalido (faltando topdesk:) em $f"; exit 1; }

  # chaves obrigatorias do bloco topdesk (estrutura esperada pelo modulo/backend)
  local required=(
    contract operator oper_group main_caller secundary_caller sla category
    sub_category call_type send_more_info more_info_text adicional_cresol
    send_email email_to email_cc
  )
  local k
  for k in "${required[@]}"; do
    grep -Eq "^[[:space:]]+$k:" "$f" || {
      echo "ERRO: YAML invalido (faltando chave '$k') em $f"
      exit 1
    }
  done
}

validate_smtp_env() {
  local f="$1"
  check_file "$f"

  if [ ! -s "$f" ]; then
    echo "ERRO: arquivo SMTP vazio: $f"
    exit 1
  fi

  local required=(
    TOPDESK_SMTP_HOST
    TOPDESK_SMTP_PORT
    TOPDESK_SMTP_USER
    TOPDESK_SMTP_PASS
    TOPDESK_SMTP_FROM
  )

  local k
  for k in "${required[@]}"; do
    grep -Eq "^[[:space:]]*${k}=" "$f" || {
      echo "ERRO: env SMTP invalido (faltando ${k}=...) em $f"
      exit 1
    }
  done
}

ensure_configs() {
  echo
  echo "==> Instalando configuracoes (/etc/zabbix/godesk)"
  check_dir "$SRC_DIST"
  check_file "$SRC_CFG_YAML"
  check_file "$SRC_CFG_SMTP"

  echo "validando arquivos de configuracao de origem..."
  validate_yaml_config "$SRC_CFG_YAML"
  validate_smtp_env "$SRC_CFG_SMTP"

  echo "criando diretorio de configuracao..."
  mkdir -p "$DEST_CFG_DIR"

  echo "copiando arquivos de configuracao..."
  cp -f "$SRC_CFG_YAML" "$DEST_CFG_YAML"
  cp -f "$SRC_CFG_SMTP" "$DEST_CFG_SMTP"

  echo "ajustando ownership..."
  chown root:root "$DEST_CFG_DIR" "$DEST_CFG_YAML" "$DEST_CFG_SMTP"

  echo "ajustando permissoes (777 conforme solicitado)..."
  chmod 777 "$DEST_CFG_DIR"
  chmod 777 "$DEST_CFG_YAML" "$DEST_CFG_SMTP"

  echo "validando arquivos instalados..."
  validate_yaml_config "$DEST_CFG_YAML"
  validate_smtp_env "$DEST_CFG_SMTP"

  echo "ok: configs instaladas em $DEST_CFG_DIR"
}

install_module() {
  echo
  echo "==> Instalando modulo do frontend"
  check_dir "$SRC_MODULE"

  echo "removendo versao antiga do modulo..."
  rm -rf "$DEST_MODULE"

  echo "instalando nova versao do modulo..."
  mkdir -p "$(dirname "$DEST_MODULE")"
  cp -r "$SRC_MODULE" "$DEST_MODULE"

  echo "ajustando permissoes do modulo..."
  chown -R root:root "$DEST_MODULE"
  find "$DEST_MODULE" -type d -exec chmod 755 {} \;
  find "$DEST_MODULE" -type f -exec chmod 644 {} \;

  echo "validando estrutura do modulo..."
  check_file "$DEST_MODULE/manifest.json"
  check_file "$DEST_MODULE/Module.php"
  check_file "$DEST_MODULE/actions/ConfigEdit.php"
  check_file "$DEST_MODULE/actions/ConfigView.php"
  check_file "$DEST_MODULE/views/godesk.configedit.php"
  check_file "$DEST_MODULE/views/godesk.configview.php"
  check_file "$DEST_MODULE/assets/css/godesk.css"
  check_file "$DEST_MODULE/assets/js/godesk.js"

  echo "ok: modulo instalado: $DEST_MODULE"
}

find_binary_candidate() {
  if [ -f "$SRC_DIST/godesk" ]; then
    printf '%s\n' "$SRC_DIST/godesk"
    return 0
  fi

  # tenta primeiro executaveis dentro de dist
  local c
  c="$(find "$SRC_DIST" -maxdepth 1 -type f -executable ! -name '*.yaml' ! -name '*.yml' ! -name '*.env' ! -name '*.json' | head -n 1 || true)"
  if [ -n "$c" ]; then
    printf '%s\n' "$c"
    return 0
  fi

  # fallback: arquivo "maior" que nao seja config
  c="$(find "$SRC_DIST" -maxdepth 1 -type f ! -name '*.yaml' ! -name '*.yml' ! -name '*.env' ! -name '*.json' -printf '%s %p\n' 2>/dev/null | sort -nr | head -n 1 | awk '{print $2}' || true)"
  if [ -n "$c" ]; then
    printf '%s\n' "$c"
    return 0
  fi

  return 1
}

install_binary() {
  echo
  echo "==> Instalando binario do goDesk (alertscripts)"
  check_dir "$SRC_DIST"

  local BIN_CANDIDATE=""
  if ! BIN_CANDIDATE="$(find_binary_candidate)"; then
    echo "ERRO: nao encontrei binario valido em $SRC_DIST"
    exit 1
  fi

  check_file "$BIN_CANDIDATE"
  echo "binario encontrado: $BIN_CANDIDATE"

  echo "criando diretorio de destino do binario..."
  mkdir -p "$DEST_BIN_DIR"

  echo "copiando binario para $DEST_BIN ..."
  cp -f "$BIN_CANDIDATE" "$DEST_BIN"

  echo "ajustando permissoes do binario..."
  chown root:root "$DEST_BIN"
  chmod 755 "$DEST_BIN"

  if [ ! -x "$DEST_BIN" ]; then
    echo "ERRO: binario nao esta executavel: $DEST_BIN"
    exit 1
  fi

  echo "validando execucao do binario..."
  set +e
  "$DEST_BIN" --help >/dev/null 2>&1
  local HELP_RC=$?
  set -e
  if [ "$HELP_RC" -eq 0 ]; then
    echo "ok: --help executou"
  else
    echo "ok: binario esta executavel (sem validacao de --help)"
  fi

  echo "ok: binario instalado: $DEST_BIN"
}

echo
echo "Selecione o que deseja instalar:"
echo "  1) Apenas modulo (frontend)"
echo "  2) Apenas binario + configs (alertscripts)"
echo "  3) Ambos (modulo + binario + configs)"
echo "  4) Apenas configs (/etc/zabbix/godesk)"
echo
read -r -p "Opcao [1-4]: " OPT

case "${OPT:-}" in
  1)
    install_module
    ;;
  2)
    ensure_configs
    install_binary
    ;;
  3)
    ensure_configs
    install_module
    install_binary
    ;;
  4)
    ensure_configs
    ;;
  *)
    echo "ERRO: opcao invalida. Use 1, 2, 3 ou 4."
    exit 1
    ;;
esac

echo
echo "ok: goDesk instalado com sucesso"
echo
echo "Frontend (modulo): $DEST_MODULE"
echo "Binario (alertscripts): $DEST_BIN"
echo "Configs: $DEST_CFG_DIR"
echo
echo "Agora no Zabbix:"
echo "Administration -> Modules -> Scan directory"
echo "Disable/Enable goDesk se precisar"
echo "Ctrl+F5 no navegador"
echo
echo "No Media Type, aponte o Script name para: $DEST_BIN"
