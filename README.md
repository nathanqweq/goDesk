# goDesk

Integração entre **Zabbix** e **TopDesk**: cria, atualiza e fecha chamados automaticamente a partir de alertas do Zabbix, com regras de roteamento (urgência, impacto, operador, cliente etc.) configuráveis por um módulo dentro do próprio frontend do Zabbix.

- `godesk` — binário Go que processa os alertas e fala com a API do TopDesk. Roda como *alertscript* (one-shot, invocado pelo Zabbix) ou como serviço systemd residente (`godesk serve`, HTTP local).
- `godesk-client` — módulo frontend do Zabbix (editor/visualizador do YAML de regras) + um binário auxiliar que sincroniza esse YAML com o `godesk serve`, para quando o frontend e o servidor estão em hosts diferentes.

Mais detalhes de arquitetura em [module.md](module.md).

## Instalação (via apt)

Repositório APT próprio, hospedado no GitHub Pages deste projeto.

### 1. Adicionar o repositório (uma vez, em cada host)

```bash
curl -fsSL https://nathanqweq.github.io/goDesk/KEY.gpg | sudo gpg --dearmor -o /usr/share/keyrings/godesk-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/godesk-archive-keyring.gpg] https://nathanqweq.github.io/goDesk stable main" | sudo tee /etc/apt/sources.list.d/godesk.list
sudo apt update
```

### 2. Instalar os pacotes certos em cada host

goDesk tem dois pacotes porque o **servidor** (que fala com o TopDesk) e o **frontend do Zabbix** (onde se edita as regras) podem estar em hosts diferentes. Se for tudo no mesmo host, instale os dois ali.

**No host que roda o Zabbix server / alertscripts:**
```bash
sudo apt install godesk
```
Instala o binário (`/usr/lib/zabbix/alertscripts/godesk`, com symlink em `/usr/bin/godesk`) e o serviço systemd `godesk.service` (já habilitado e iniciado).

**No host que roda o frontend web do Zabbix:**
```bash
sudo apt install godesk-client
```
Instala o módulo do Zabbix (`/usr/share/zabbix/ui/modules/goDesk`) e o binário `godesk-client`, que envia o YAML de regras para o `godesk serve` sempre que alguém salva a config pela UI.

### 3. Configurar

Arquivos em `/etc/zabbix/godesk/` (criados com valores de exemplo na instalação — edite antes de usar):

| Arquivo | Onde | Para quê |
|---|---|---|
| `godesk-config.yaml` | host do servidor | regras de roteamento (urgência, impacto, operador, cliente) — normalmente editado pela UI do Zabbix, não à mão |
| `godesk-smtp-config.env` | host do servidor | credenciais SMTP, se `send_email` estiver ativo em alguma regra |
| `godesk-service.env` | host do servidor | `GODESK_LISTEN_ADDR`/`GODESK_SERVICE_TOKEN` do `godesk serve`, TopDesk padrão (`GODESK_TOPDESK_DOMAIN/USER/PASS`), Zabbix padrão (`GODESK_ZABBIX_URL/KEY`) e intervalo do healthcheck (`GODESK_HEALTHCHECK_INTERVAL`) |
| `godesk-client.env` | host do frontend (só se `godesk-client` estiver instalado) | `GODESK_SERVER_URL` e `GODESK_SERVICE_TOKEN` — precisa bater com o token do `godesk-service.env` do servidor |

Depois de editar `godesk-service.env`, reinicie o serviço:
```bash
sudo systemctl restart godesk
```

### 4. Habilitar o módulo no Zabbix

No frontend do Zabbix: **Administration → General → Modules → Scan directory**, depois habilite o **goDesk**. O menu aparece em **Alerts → goDesk** (visualizar/editar config).

### 5. Apontar o Media Type do Zabbix

Em **Alerts → Media types**, configure um Media Type do tipo Script apontando para `/usr/lib/zabbix/alertscripts/godesk`, com **apenas 2 parâmetros, nesta ordem**: `TICKET_NAME RAWDATA` (veja [dist/message.json](dist/message.json) para o template do `RAWDATA`).

> **Atenção, mudança de versão:** até a `v1.2.x` o Media Type precisava de 7 parâmetros (`DOMAIN USER PASS TICKET_NAME RAWDATA ZABBIX_URL ZABBIX_KEY`). A partir daqui `DOMAIN`/`USER`/`PASS`/`ZABBIX_URL`/`ZABBIX_KEY` **não são mais parâmetros do Media Type** — vêm exclusivamente de `GODESK_TOPDESK_DOMAIN/USER/PASS` e `GODESK_ZABBIX_URL/KEY` em `godesk-service.env`, carregados uma vez quando o `godesk`/`godesk serve` inicia. Se seu Media Type ainda estiver com os 7 parâmetros antigos, o `godesk` vai interpretar `DOMAIN` como se fosse `TICKET_NAME` — **edite o Media Type pra deixar só `TICKET_NAME RAWDATA`, nessa ordem, antes de atualizar**.

### 6. (Opcional) Healthcheck do TopDesk

Com `GODESK_TOPDESK_DOMAIN/USER/PASS` e `GODESK_HEALTHCHECK_INTERVAL` configurados em `godesk-service.env`, o `godesk serve` testa `GET /tas/api/incidents?pageSize=1` nesse intervalo, em background (não interfere no processamento de alertas), registrando sucesso/erro e latência nos logs e em `godesk --monitoring`.

## Instalação manual (sem apt)

Use [install.sh](install.sh) — compila/copia os arquivos na mão, com as mesmas opções (módulo, binário, configs, serviço systemd). Útil se o host não tem acesso ao repositório APT.

## Empacotamento e releases

Detalhes de como os `.deb` são gerados e publicados (chave GPG, GitHub Actions, GitHub Pages) em [packaging/README.md](packaging/README.md).
