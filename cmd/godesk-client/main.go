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
	if len(os.Args) > 1 && os.Args[1] == "validate" {
		if err := runValidate(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "godesk-client: "+err.Error())
			os.Exit(1)
		}
		return
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

	configPath := resolve(env, "TOPDESK_CONFIG", "/etc/zabbix/godesk/godesk-config.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("falha ao ler %s: %w", configPath, err)
	}

	if _, err := config.ParsePolicies(data); err != nil {
		return fmt.Errorf("yaml inválido em %s: %w", configPath, err)
	}

	req, err := http.NewRequest(http.MethodPost, serverURL+"/config", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/x-yaml")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("falha ao enviar para %s: %w", serverURL, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("servidor respondeu %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	fmt.Printf("config enviada com sucesso para %s\n", serverURL)
	return nil
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
