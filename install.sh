#!/usr/bin/env bash
set -euo pipefail

echo "=== goDesk module + binary installer ==="

# caminho do projeto (onde está Module/goDesk e dist/)
PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"

# --- FRONTEND MODULE ---
SRC_MODULE="$PROJECT_ROOT/Module/goDesk"
DEST_MODULE="/usr/share/zabbix/ui/modules/goDesk"

# --- BINARY (Zabbix alertscripts) ---
SRC_DIST="$PROJECT_ROOT/dist"
DEST_BIN_DIR="/usr/lib/zabbix/alertscripts"
DEST_BIN="$DEST_BIN_DIR/godesk"

# precisa ser root
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
    echo "ERRO: faltando diretório $1"
    exit 1
  fi
}

install_module() {
  echo
  echo "==> Instalando módulo do frontend"
  check_dir "$SRC_MODULE"

  echo "removendo versão antiga do módulo..."
  rm -rf "$DEST_MODULE"

  echo "instalando nova versão do módulo..."
  cp -r "$SRC_MODULE" "$DEST_MODULE"

  echo "ajustando permissoes do módulo..."
  chown -R root:root "$DEST_MODULE"
  find "$DEST_MODULE" -type d -exec chmod 755 {} \;
  find "$DEST_MODULE" -type f -exec chmod 644 {} \;

  echo "validando estrutura do módulo..."
  check_file "$DEST_MODULE/manifest.json"
  check_file "$DEST_MODULE/Module.php"
  check_file "$DEST_MODULE/actions/ConfigEdit.php"
  check_file "$DEST_MODULE/actions/ConfigView.php"
  check_file "$DEST_MODULE/views/godesk.configedit.php"
  check_file "$DEST_MODULE/views/godesk.configview.php"
  check_file "$DEST_MODULE/assets/css/godesk.css"
  check_file "$DEST_MODULE/assets/js/godesk.js"

  echo "✅ Módulo instalado: $DEST_MODULE"
}

install_binary() {
  echo
  echo "==> Instalando binário do goDesk (alertscripts)"
  check_dir "$SRC_DIST"

  # tenta achar o binário dentro de dist
  # prioridade: dist/godesk, depois primeiro arquivo regular encontrado
  local BIN_CANDIDATE=""
  if [ -f "$SRC_DIST/godesk" ]; then
    BIN_CANDIDATE="$SRC_DIST/godesk"
  else
    BIN_CANDIDATE="$(find "$SRC_DIST" -maxdepth 1 -type f | head -n 1 || true)"
  fi

  if [ -z "$BIN_CANDIDATE" ]; then
    echo "ERRO: não encontrei nenhum binário em $SRC_DIST"
    exit 1
  fi

  echo "binário encontrado: $BIN_CANDIDATE"

  echo "criando diretório de destino do binário..."
  mkdir -p "$DEST_BIN_DIR"

  echo "copiando binário para $DEST_BIN ..."
  cp -f "$BIN_CANDIDATE" "$DEST_BIN"

  echo "ajustando permissões do binário..."
  chown root:root "$DEST_BIN"
  chmod 755 "$DEST_BIN"

  if [ ! -x "$DEST_BIN" ]; then
    echo "ERRO: binário não está executável: $DEST_BIN"
    exit 1
  fi

  echo "validando execução do binário..."
  set +e
  "$DEST_BIN" --help >/dev/null 2>&1
  local HELP_RC=$?
  set -e
  if [ $HELP_RC -eq 0 ]; then
    echo "ok: --help executou"
  else
    echo "ok: binário é executável (sem validação de --help)"
  fi

  echo "✅ Binário instalado: $DEST_BIN"
}

echo
echo "Selecione o que deseja instalar:"
echo "  1) Apenas módulo (frontend)"
echo "  2) Apenas binário (alertscripts)"
echo "  3) Ambos (módulo + binário)"
echo
read -r -p "Opção [1-3]: " OPT

case "${OPT:-}" in
  1)
    install_module
    ;;
  2)
    install_binary
    ;;
  3)
    install_module
    install_binary
    ;;
  *)
    echo "ERRO: opção inválida. Use 1, 2 ou 3."
    exit 1
    ;;
esac

echo
echo "✅ goDesk instalado com sucesso"
echo
echo "Frontend (módulo): $DEST_MODULE"
echo "Binário (alertscripts): $DEST_BIN"
echo
echo "Agora no Zabbix:"
echo "Administration → Modules → Scan directory"
echo "Disable/Enable goDesk se precisar"
echo "Ctrl+F5 no navegador"
echo
echo "No Media Type, aponte o Script name para: $DEST_BIN"
