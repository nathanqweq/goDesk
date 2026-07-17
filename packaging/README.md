# Empacotamento APT do goDesk

## Instalar (usuário final)

```bash
curl -fsSL https://nathanqweq.github.io/goDesk/KEY.gpg | sudo gpg --dearmor -o /usr/share/keyrings/godesk-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/godesk-archive-keyring.gpg] https://nathanqweq.github.io/goDesk stable main" | sudo tee /etc/apt/sources.list.d/godesk.list
sudo apt update
sudo apt install godesk
```

Isso instala:
- `/usr/lib/zabbix/alertscripts/godesk` (binário, com symlink em `/usr/bin/godesk`)
- `/usr/lib/systemd/system/godesk.service` (habilitado e iniciado automaticamente)
- `/etc/zabbix/godesk/{godesk-config.yaml,godesk-smtp-config.env,godesk-service.env}` (conffiles — edições locais são preservadas em upgrades)

O módulo frontend do Zabbix ([Module/goDesk](../Module/goDesk)) não faz parte deste pacote; continue instalando-o com `install.sh` (opção 1).

### godesk-client (só quando frontend e servidor estão em hosts separados)

O módulo Zabbix (PHP) salva `godesk-config.yaml` no disco do host que roda o **frontend**. Se o `godesk serve` roda em outro host (topologia comum: Zabbix server separado do frontend web), esse arquivo nunca chega até ele. O `godesk-client` resolve isso: instale-o **no host do frontend**, ele valida o YAML salvo e envia pro `godesk serve` remoto automaticamente toda vez que o admin salva a config pela UI.

```bash
sudo apt install godesk-client
sudo nano /etc/zabbix/godesk/godesk-client.env   # GODESK_SERVER_URL + GODESK_SERVICE_TOKEN (mesmo token do dist/godesk-service.env do servidor)
```

Sem esse pacote instalado, o comportamento continua exatamente como antes (save só local) — o módulo detecta a ausência do binário e não tenta sincronizar.

## Publicar uma nova versão (mantenedor)

### Setup único
1. Rode `packaging/gen-apt-key.sh` **localmente, na sua máquina** (nunca em CI ou em um assistente) para gerar o par de chaves GPG. Ele gera `packaging/KEY.gpg` (chave pública, pode commitar) e `packaging/.secrets/` (chave privada + passphrase, **não commitar** — já está no `.gitignore`).
2. Cadastre os dois segredos em GitHub → Settings → Secrets and variables → Actions:
   - `APT_GPG_PRIVATE_KEY`: conteúdo de `packaging/.secrets/private-key.b64`
   - `APT_GPG_PASSPHRASE`: conteúdo de `packaging/.secrets/passphrase.txt`
   Depois apague `packaging/.secrets/` localmente.
3. Ative o GitHub Pages: Settings → Pages → Source = branch `gh-pages` / root. O branch `gh-pages` é criado automaticamente pela Action no primeiro release; ative o Pages depois desse primeiro run (ou deixe pendente — a Action funciona igual, só o site demora a aparecer).

### A cada release
```bash
git tag v1.0.0
git push origin v1.0.0
```
O workflow [.github/workflows/publish-apt.yml](../.github/workflows/publish-apt.yml) builda os dois `.deb` (`godesk` e `godesk-client`), atualiza o repositório APT no branch `gh-pages` e anexa ambos também à Release do GitHub.

### Testar o build localmente (sem publicar)
Requer Go + `dpkg-dev` (`sudo apt install dpkg-dev`) em Linux:
```bash
packaging/build-deb.sh 0.0.0-test
dpkg-deb --info build/godesk_0.0.0-test_amd64.deb
dpkg-deb --contents build/godesk_0.0.0-test_amd64.deb

packaging/build-deb-client.sh 0.0.0-test
dpkg-deb --contents build/godesk-client_0.0.0-test_amd64.deb
```
