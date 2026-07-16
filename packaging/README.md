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
O workflow [.github/workflows/publish-apt.yml](../.github/workflows/publish-apt.yml) builda o `.deb`, atualiza o repositório APT no branch `gh-pages` e anexa o `.deb` também à Release do GitHub.

### Testar o build localmente (sem publicar)
Requer Go + `dpkg-dev` (`sudo apt install dpkg-dev`) em Linux:
```bash
packaging/build-deb.sh 0.0.0-test
dpkg-deb --info build/godesk_0.0.0-test_amd64.deb
dpkg-deb --contents build/godesk_0.0.0-test_amd64.deb
```
