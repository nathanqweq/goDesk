
# 🧠 goDesk Zabbix Module

### Integração Zabbix + goDesk (TopDesk automation module)

Módulo frontend para Zabbix que permite:

- Visualizar e editar configuração YAML de integração
    
- Automação de chamados (TopDesk/goDesk)
    
- Definição de urgência/impacto por cliente
    
- Autoclose automático
    
- Integração com scripts Go
    

---

# 📦 Arquitetura geral

```
Zabbix Frontend (PHP Module)
        │
        ├── YAML config (/etc/zabbix/godesk/)
        │
        ├── GoDesk Go Binary
        │
        └── Integração TopDesk API
```

O módulo funciona como **central de automação de chamados** dentro do Zabbix.

---

# 📁 Estrutura do módulo Zabbix

Instalar em:

```
/usr/share/zabbix/ui/modules/godesk
```

Estrutura:

```
godesk/
 ├── manifest.json
 ├── Module.php
 ├── actions/
 │    ├── ConfigView.php
 │    └── ConfigEdit.php
 │
 ├── views/
 │    ├── godesk.configview.php
 │    └── godesk.configedit.php
 │
 └── assets/
      └── css/
           └── godesk.css
```

---

# ⚙️ Requisitos

## Sistema

- Zabbix 6.4+
    
- PHP 8.1+
    
- nginx ou apache
    
- Linux server
    

## Extensões PHP obrigatórias

```bash
sudo apt install php-yaml
```

Reiniciar:

```bash
systemctl restart php8.2-fpm
systemctl restart nginx
```

Verificar:

```bash
php -m | grep yaml
```

---

# 🔐 Permissões necessárias

Criar diretório config:

```bash
sudo mkdir -p /etc/zabbix/godesk
sudo chown -R nginx:nginx /etc/zabbix/godesk
sudo chmod 750 /etc/zabbix/godesk
```

Arquivo principal:

```
/etc/zabbix/godesk/godesk-config.yaml
```

Permissão:

```bash
sudo chmod 640 /etc/zabbix/godesk/godesk-config.yaml
```

---

# 🧾 Estrutura do YAML

```yaml
default:
  urgency: "Baixa"
  impact: "Sem impacto"
  autoclose: false
  tags:
    contract: "DEFAULT"
    oper_group: "GROUP"
    main_caller: "email@empresa.com"
    secundary_caller: ""

clients:
  Cresol:
    autoclose: true
    urgency: "Alta"
    impact: "Indisponibilidade"
    tags:
      contract: "Cresol"
      oper_group: "NOC"
      main_caller: "noc@cliente.com"
      secundary_caller: ""
```

---

# 🚀 Instalação do módulo

## 1. Copiar módulo

```bash
cd /usr/share/zabbix/ui/modules
git clone https://repo/godesk.git
```

ou copiar pasta manualmente.

---

## 2. Ativar no Zabbix

Menu:

```
Administration → General → Modules
```

Clique:

```
Scan directory
Enable goDesk
```

---

# 🖥️ Telas do módulo

## Visualização

```
Monitoring → goDesk → Config visualizar
```

Mostra:

- default config
    
- clientes
    
- tags
    
- autoclose
    

## Edição

```
Monitoring → goDesk → Config editar
```

Permite:

- editar YAML via form
    
- adicionar clientes
    
- remover clientes
    
- salvar com backup automático
    

Backup criado:

```
/etc/zabbix/godesk/godesk-config.yaml.bak.DATA
```

---

# 🎨 CSS e temas

CSS localizado em:

```
assets/css/godesk.css
```

Compatível com:

- light theme
    
- dark theme
    
- high contrast
    

Usa escopo:

```
.godesk-module
```

Para evitar conflito com CSS do Zabbix.

---

# 🐹 Compilação do binário Go (goDesk engine)

Estrutura:

```
cmd/godesk/main.go
internal/
```

Compilar no Windows para Linux:

```powershell
$env:CGO_ENABLED="0"
$env:GOOS="linux"
$env:GOARCH="amd64"

go build -o godesk ./cmd/godesk
```

Enviar ao servidor:

```bash
scp godesk root@server:/usr/local/bin/
chmod +x /usr/local/bin/godesk
```

---

# 🔍 Troubleshooting

## CSS não carrega

- verificar manifest.json assets
    
- F12 → network → godesk.css
    
- ctrl+F5
    

## YAML não salva

verificar permissão:

```bash
ls -l /etc/zabbix/godesk
```

PHP deve ter write.

## yaml_parse erro

instalar:

```bash
apt install php-yaml
```

---

# 🧠 Roadmap futuro

## v1.1

- validação YAML
    
- botão testar config
    
- logs
    

## v1.2

- integração TopDesk API
    
- criação automática de chamados
    
- auto-close
    

## v2.0 (enterprise)

- engine Go integrada
    
- filas async
    
- cache redis
    
- multi-tenant
    

---

# 👨‍💻 Autor

Nathan Quadros / Teltec Solutions

Projeto interno de automação de chamados e integração Zabbix + goDesk.