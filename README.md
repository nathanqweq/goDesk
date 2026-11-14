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
**IMPORTANTE** Caso seu Zabbix Server e Zabbix Front estejam na mesma máquina, o arquivo **godesk-service.env** não é necessário configurar, pois o servidor vai conseguir interpretar os dados localmente.

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

### 7. (Opcional) Notificação no MS Teams

Marque **Enviar Teams** na rule (pela UI do módulo) e preencha os campos abaixo. O Teams é sempre configurado **por rule**, com ou sem cliente escolhido: cada rule liga, desliga e escolhe o próprio canal. O que estiver no **default** vale para alertas cuja rule não está cadastrada, e o webhook/URL do default servem de padrão para rules que deixarem esses campos vazios.

- **Webhook do Teams (Workflow)**: URL de um Workflow do Teams criado a partir do template *"Post a message in a channel when a webhook request is received"*.
- **URL do Zabbix (frontend)**: usada no botão *Event info* do card. Se ficar vazia, usa `GODESK_ZABBIX_URL`.

Com isso, o goDesk posta um card no canal (mesmo formato do media type *MS Teams Workflow* do Zabbix, já com o número do chamado do TopDesk) quando cria o chamado e quando recebe a normalização de um chamado existente. Uma falha no envio só gera log (`[teams]`) e a métrica `teams_send_errors_total`. O chamado não é afetado.

### 8. (Opcional) Dependência entre hosts

Evita abrir um chamado por servidor quando o problema real é o switch ou firewall de que eles dependem. Não usa a dependência nativa de triggers do Zabbix e não exige trigger nova nem mudança no Media Type.

1. Em **Alerts → goDesk → Dependências**, cadastre cada host dependente e os hosts pai dele:
   - **Modo**: *qualquer* não abre chamado se pelo menos um pai estiver em alerta. *Todos* só segura o chamado se todos os pais estiverem em alerta (para links e switches redundantes).
   - **Evento a observar** (por pai): nome do problema no pai. Aceita `*` como curinga, por exemplo `*Interface*down*`. Se ficar vazio, qualquer problema ativo do pai conta. Ao focar o campo, ele sugere as triggers do host pai.
   - **Exportar/Importar**: baixa ou carrega o `godesk-dependencies.yaml`. Importar **substitui** tudo, e um backup do arquivo anterior fica salvo. Na importação, o hostid é recalculado pelo nome do host, então o arquivo funciona entre ambientes diferentes.
2. Marque **Dependência** nas rules que devem respeitar isso.

As dependências ficam em `/etc/zabbix/godesk/godesk-dependencies.yaml` (caminho alterável com `GODESK_DEPENDENCIES_FILE`), um arquivo separado do `godesk-config.yaml`. Ao salvar pela UI, ele é sincronizado com o servidor pelo mesmo caminho da config (`godesk-client dependencies` → `POST /dependencies`).

Quando um alerta de uma rule com dependência chega para um host cadastrado e não existe chamado aberto, o goDesk consulta os problemas ativos dos pais na API do Zabbix (`problem.get`). Se o pai estiver em alerta, o chamado **não é aberto**. Em vez disso, o goDesk comenta no evento do Zabbix (*"host X depende de host(s) com alerta ativo: ..."*) e incrementa a métrica `alerts_held_by_dependency_total`.

- O comentário **não reconhece** o evento. Configure um **passo de escalonamento** na Action do Zabbix (por exemplo, reenviar após 15 min, com a condição *Problem is not acknowledged*). Assim, se o pai normalizar e o host continuar com problema, o reenvio abre o chamado normalmente.
- Se der erro na consulta (Zabbix fora do ar, sem permissão, host pai inexistente), o chamado é aberto normalmente.
- O token em `GODESK_ZABBIX_KEY` precisa de permissão de leitura nos hosts pai.

### 9. (Opcional) Dependência v2: arquitetura por localidade

Roda **em paralelo** à dependência do item 8 (v1), sem alterá-la. Em vez de cadastrar pai por host, você desenha a arquitetura de cada localidade, e o goDesk decide quem é causa raiz.

1. Em **Alerts → goDesk → Dependências v2 (arquitetura)**, crie uma localidade e preencha:
   - **Host de status** e **Evento de status** (ex: `UHCC - Core Status` / `Powerplant Down`). Com esse evento ativo, **nenhum host da localidade abre chamado**, só o próprio host de status.
   - **Root** (padrão `ZABBIX`): o ponto de entrada do monitoramento. Ligue-o aos equipamentos de entrada (os SDs).
   - **Evento de DOWN** (padrão `*Unavailable by ICMP ping*`): o problema que marca um host como caído.
   - **Hosts e links**: use **Importar hosts de um grupo** (o grupo raiz da usina) e desenhe os cabos no modo **🔗 Ligar**. Marque o rack de cada host.
2. Use **🧪 Simular** para marcar hosts como DOWN (ou carregar o estado atual do Zabbix) e ver quem abriria chamado antes de ligar a v2.
3. Em **Config (editar)**, escolha a **Versão da dependência** da rule (precisa estar com **Dependência** marcada):
   - **v1**: comportamento de sempre.
   - **shadow**: a v1 continua decidindo, e a v2 só registra no log o que teria feito (`[dependency-shadow] IGUAL|DIVERGE ...`) e conta divergências em `dependency_shadow_diverged_total`. Use para validar em produção sem risco.
   - **v2**: a v2 decide. Host fora de qualquer localidade da v2 cai na v1.

Como a v2 decide, na hora do alerta:
- Primeiro checa o **status da localidade**: se o evento de status estiver ativo, segura (exceto o host de status).
- Depois a **topologia**: o host só abre chamado se ainda existir caminho do root até um vizinho dele passando apenas por hosts UP. Por isso a redundância é respeitada (um FW com perna nos dois membros do core só é segurado se os dois caírem), e o comentário no evento informa quem cortou o caminho e o rack.
- Host sem links, host não encontrado no Zabbix ou erro de consulta: abre chamado (mesma regra de segurança da v1).

**Visualização para o atendimento:** **Alerts → goDesk → Arquitetura (visualizar)** (`godesk.topology.view`) mostra as arquiteturas em modo somente leitura: carrega o estado atual do Zabbix ao abrir e permite simular quedas clicando nos hosts, sem poder editar ou salvar. Funciona para qualquer usuário logado cujo role tenha o módulo goDesk liberado (**Users → User roles → Access to modules**). As telas de edição continuam restritas ao Super admin e nem aparecem no menu dos demais. O estado atual respeita as permissões do usuário: um host que ele não enxerga no Zabbix aparece como UP.

As localidades ficam em `/etc/zabbix/godesk/godesk-dependencies-v2.yaml` (caminho alterável com `GODESK_DEPENDENCIES_V2_FILE`), sincronizado pelo `godesk-client dependencies-v2` → `POST /dependencies-v2`. Exportar/Importar funcionam como na v1. O estado da localidade é lido com 3 chamadas à API do Zabbix por alerta (`host.get`, `problem.get`, `trigger.get`), independente do número de hosts.

### 10. (Opcional) Fluxos: decisão por condições, com poucas rules

Em vez de criar uma rule nova para cada variação ("host crítico liga", "fora do horário liga sempre", "informativo não abre"), a rule aponta para um **fluxo**, e o fluxo decide na hora do alerta.

1. Em **Alerts → goDesk → Fluxos**, crie um fluxo e monte os blocos:
   - **Condição** (sim/não), com regras E/OU sobre o evento. Regras podem ser agrupadas: `(tag = Critico OU grupo começa com Core/) E fora do horário` (até 3 níveis).
   - **Condição múltipla** (se / senão se / senão): vários ramos, cada um com suas regras, testados em ordem. O primeiro que valer escolhe a saída; se nenhum valer, sai pelo "senão".
   - **Transformar texto**: cria variáveis `{var.nome}` a partir de um campo, com estas operações: extrair por regex (por exemplo, a interface do nome do trigger), maiúsculas, minúsculas, substituir, aparar espaços ou valor padrão quando vier vazio. As variáveis valem nas condições e nos textos dos blocos seguintes.
   - **Switch**, com um caminho por valor. Exemplo: `host.tag.Criticidade` = `Critico`, `Alta,Media` ou outro.
   - **Tabela**: a primeira linha que bater com um campo (ex: `host.group` = `POWERPLANT/*`) define várias colunas de uma vez (grupo operador, contrato, webhook do Teams...). Saídas *achou* / *não achou*.
   - **Definir campos**: urgência, impacto, grupo operador, operador, contrato, Teams (`teams_webhook` aceita vários, separados por vírgula; `teams_title` e `teams_message` personalizam o card), e-mail etc. e a **instrução** pro atendimento (ex: *"LIGAR para o cliente ({host})"*).
   - **Atualizar chamado** (escalonamento): veja abaixo.
   - **HTTP**: chama outro sistema depois de tratar o chamado (método, URL, cabeçalhos e corpo com variáveis; `{ticket.number}` é o número do chamado). Só no modo ativo, e o resultado aparece no Histórico.
   - **Fim**: abrir chamado, ou **não abrir** (só comenta no evento). Uma saída sem ligação também abre o chamado com o que já foi definido.
2. Em **Calendários**, cadastre por exemplo `comercial` (seg a sex, 08:00–18:00, feriados). Nas condições, use `calendar.comercial` "é falso" para pegar o que está fora do horário.
3. Use **🧪 Testar** com um problema real (ou valores digitados): a tela mostra o caminho, os campos definidos e se abre ou não. É a mesma decisão do goDesk.
4. Em **Config (editar)**, preencha o **Fluxo** da rule e o **Modo do fluxo**:
   - **ativo**: o fluxo decide.
   - **shadow**: a rule fixa decide, e o fluxo só registra no log o que faria (`[flow-shadow] ... caminho: ... | definiria: ...`).

Campos disponíveis nas condições e nos textos (`{campo}`):

| Campo | O que é |
|---|---|
| `host`, `host.name`, `host.id` | host do alerta (nome técnico, visível, id) |
| `host.group` | grupos do host (vários valores) |
| `host.tag.<nome>`, `event.tag.<nome>` | tags do host e do evento |
| `trigger`, `severity`, `severity.level` (0–5), `value`, `status`, `event.kind` (`problem`/`recovery`) | dados do evento |
| `event.age` (minutos desde o início), `event.resends` (quantas vezes o alerta já chegou antes; 0 = primeira), `event.acked_by_user` (reconhecido por alguém do atendimento), `event.acknowledged` (por qualquer um, inclusive o goDesk) | escalonamento |
| `rule`, `cliente`, `equipe`, `ticket.exists`, `ticket.status`, `ticket.number` | contexto do goDesk |
| `calendar.<nome>`, `now.weekday`, `now.hour`, `now.time`, `now.date` | horário |
| `set.<campo>` | valor já definido antes no próprio fluxo |
| `var.<nome>` | variável criada pelo bloco Transformar texto |

Operadores das regras: igual, diferente, contém, corresponde (`*`), está na lista, preenchido/vazio, maior/menor, verdadeiro/falso, **começa com**, **termina com**, **regex** (sem diferenciar maiúsculas; sintaxe RE2, sem lookahead) e **entre** (`30..60` para números; `22:00..06:00` para horário, atravessando a meia-noite quando o início é maior que o fim).

Nos textos, `{campo|json}` escapa o valor para dentro de uma string JSON (corpo do bloco HTTP) e `{campo|url}` codifica para URL. As tags do host (`host.tag.*`) incluem as herdadas dos templates.

**Escalonamento** (bloco **Atualizar chamado**): configure na action do Zabbix passos que **reenviam** o alerta a cada X minutos. A cada reenvio com o chamado já aberto, o fluxo roda de novo e o bloco pode mudar urgência, impacto, prioridade, grupo operador ou operador, registrar uma nota (invisível para o solicitante) e avisar no Teams. Por padrão, cada bloco roda **uma vez por evento**; marque *repetir* para rodar a cada reenvio. Exemplo: Condição `event.age ≥ 30` E `event.acked_by_user` é falso → Atualizar chamado (urgência Crítica, grupo N2, Teams). Com `repeat_update` = não (bloco Definir campos), o goDesk para de registrar *"Recebemos novamente o alerta..."* no chamado a cada reenvio. O `event.acked_by_user` ignora o ack que o próprio goDesk faz ao abrir o chamado (identificado pelas mensagens "Chamado ...").

**Histórico**: **Alerts → goDesk → Fluxos — histórico** mostra cada alerta que passou por um fluxo (ativo, shadow ou com erro): caminho, campos definidos, escalonamento, HTTP e o que o goDesk fez. Os dados ficam no servidor goDesk (`/var/lib/godesk/godesk-flows-history.jsonl`, últimas ~5000 execuções, alterável com `GODESK_FLOWS_HISTORY_FILE`) e chegam no módulo por `godesk-client flows-history` → `GET /flows/history`. Só Super admin. O estado por evento (reenvios e escalonamentos já feitos) fica em `/var/lib/godesk/godesk-flows-state.json` (`GODESK_FLOWS_STATE_FILE`) e é limpo depois de 7 dias.

**Para o atendimento**: **Alerts → goDesk → Fluxos (visualizar)** mostra os fluxos em modo somente leitura, com o 🧪 Testar. Funciona para qualquer usuário cujo role tenha o módulo goDesk liberado, e o teste usa as permissões de host desse usuário.

Regras do fluxo:
- O que o fluxo define **sobrescreve** a rule, inclusive o que vier por tag do Zabbix. Um texto que resolve vazio não apaga o valor da rule.
- A **instrução** é gravada no chamado como ação **invisível pro solicitante** e aparece em destaque no card do Teams. **Não** vai no e-mail do cliente.
- **Não abrir** só vale na abertura. A atualização e a normalização de um chamado já aberto seguem normais.
- **Atualizar chamado** só vale quando o alerta volta e o chamado já existe. Na abertura, o bloco é ignorado.
- Tags e grupos do host só são consultados no Zabbix se o fluxo chegar num bloco que use esses campos (1 `host.get` e/ou 1 `event.get` por alerta).
- Se der erro (fluxo inexistente, arquivo inválido), a rule segue fixa, como se não tivesse fluxo. Isso conta em `flows_errors_total`. Outras métricas: `flows_evaluated_total`, `flows_skipped_total` e `flows_shadow_evaluated_total`.

Os fluxos ficam em `/etc/zabbix/godesk/godesk-flows.yaml` (alterável com `GODESK_FLOWS_FILE`), gravado em JSON indentado (JSON é YAML válido). O arquivo é validado pelo próprio goDesk antes de salvar (`godesk-client flows-check`) e sincronizado por `godesk-client flows` → `POST /flows`.

## Instalação manual (sem apt)

Use [install.sh](install.sh) — compila/copia os arquivos na mão, com as mesmas opções (módulo, binário, configs, serviço systemd). Útil se o host não tem acesso ao repositório APT.

## Empacotamento e releases

Detalhes de como os `.deb` são gerados e publicados (chave GPG, GitHub Actions, GitHub Pages) em [packaging/README.md](packaging/README.md).
