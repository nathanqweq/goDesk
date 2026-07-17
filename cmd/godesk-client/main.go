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
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "godesk-client: "+err.Error())
		os.Exit(1)
	}
}

func run() error {
	env := config.LoadEnvFile(clientEnvPath)

	serverURL := strings.TrimRight(resolve(env, "GODESK_SERVER_URL", ""), "/")
	token := resolve(env, "GODESK_SERVICE_TOKEN", "")
	configPath := resolve(env, "TOPDESK_CONFIG", "/etc/zabbix/godesk/godesk-config.yaml")

	if serverURL == "" {
		return fmt.Errorf("GODESK_SERVER_URL não configurado (%s)", clientEnvPath)
	}
	if token == "" {
		return fmt.Errorf("GODESK_SERVICE_TOKEN não configurado (%s)", clientEnvPath)
	}

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
