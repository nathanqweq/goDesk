package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"godesk/internal/config"
)

const clientEnvPath = "/etc/zabbix/godesk/godesk-client.env"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "validate":
			if err := runValidate(os.Args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, "godesk-client: "+err.Error())
				os.Exit(1)
			}
			return
		case "test-config":
			if err := runTestConfig(); err != nil {
				fmt.Fprintln(os.Stderr, "godesk-client: "+err.Error())
				os.Exit(1)
			}
			return
		}
	}

	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "godesk-client: "+err.Error())
		os.Exit(1)
	}
}

// loadClientEnv lê godesk-client.env uma vez e devolve tanto o mapa
// completo (pra quem precisar de outras chaves, ex: TOPDESK_CONFIG)
// quanto GODESK_SERVER_URL/GODESK_SERVICE_TOKEN já resolvidos e validados.
func loadClientEnv() (env map[string]string, serverURL, token string, err error) {
	env = config.LoadEnvFile(clientEnvPath)

	serverURL = strings.TrimRight(resolve(env, "GODESK_SERVER_URL", ""), "/")
	token = resolve(env, "GODESK_SERVICE_TOKEN", "")

	if serverURL == "" {
		return nil, "", "", fmt.Errorf("GODESK_SERVER_URL não configurado (%s)", clientEnvPath)
	}
	if token == "" {
		return nil, "", "", fmt.Errorf("GODESK_SERVICE_TOKEN não configurado (%s)", clientEnvPath)
	}

	return env, serverURL, token, nil
}

func run() error {
	env, serverURL, token, err := loadClientEnv()
	if err != nil {
		return err
	}

	data, configPath, err := readLocalConfig(env)
	if err != nil {
		return err
	}

	if _, err := postYAML(serverURL, token, "/config", data); err != nil {
		return err
	}

	fmt.Printf("config enviada com sucesso para %s (lida de %s)\n", serverURL, configPath)
	return nil
}

// runTestConfig manda o mesmo arquivo de config real (TOPDESK_CONFIG) pro
// endpoint de dry run (/config/test, ver internal/server/configtest.go):
// valida o YAML e confere permissão de escrita no servidor sem nunca
// sobrescrever o godesk-config.yaml de produção. Serve pra testar o canal
// client→server (send + save) sem risco de aplicar um payload errado.
func runTestConfig() error {
	env, serverURL, token, err := loadClientEnv()
	if err != nil {
		return err
	}

	data, configPath, err := readLocalConfig(env)
	if err != nil {
		return err
	}

	if _, err := postYAML(serverURL, token, "/config/test", data); err != nil {
		return err
	}

	fmt.Printf("OK: %s é um YAML válido e o servidor (%s) consegue gravar — nada foi alterado\n", configPath, serverURL)
	return nil
}

// readLocalConfig lê e valida o TOPDESK_CONFIG local — reaproveitado por
// run() e runTestConfig(), que só diferem em pra qual endpoint mandam o
// mesmo conteúdo.
func readLocalConfig(env map[string]string) (data []byte, path string, err error) {
	path = resolve(env, "TOPDESK_CONFIG", "/etc/zabbix/godesk/godesk-config.yaml")

	data, err = os.ReadFile(path)
	if err != nil {
		return nil, path, fmt.Errorf("falha ao ler %s: %w", path, err)
	}

	if _, err := config.ParsePolicies(data); err != nil {
		return nil, path, fmt.Errorf("yaml inválido em %s: %w", path, err)
	}

	return data, path, nil
}

// postYAML manda data (Content-Type: application/x-yaml) pro endpoint
// informado do servidor e devolve o corpo da resposta em caso de sucesso.
func postYAML(serverURL, token, path string, data []byte) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, serverURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/x-yaml")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("falha ao enviar para %s%s: %w", serverURL, path, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("servidor respondeu %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return body, nil
}

// runValidate repassa pro servidor um JSON com campos tipo
// {"operator":"...","oper_group":"..."} e imprime a resposta (também
// JSON, com o resultado por campo) em stdout — usado pelo botão "Testar"
// do módulo Zabbix via ConfigValidate.php.
func runValidate(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("uso: godesk-client validate '<json com operator/oper_group>'")
	}

	_, serverURL, token, err := loadClientEnv()
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, serverURL+"/validate-client", strings.NewReader(args[0]))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("falha ao validar em %s: %w", serverURL, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("servidor respondeu %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	fmt.Println(string(body))
	return nil
}

// resolve segue a mesma prioridade já usada no resto do goDesk: variável de
// ambiente do processo primeiro, depois o arquivo .env, depois o default.
func resolve(env map[string]string, key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	if v := strings.TrimSpace(env[key]); v != "" {
		return v
	}
	return def
}
