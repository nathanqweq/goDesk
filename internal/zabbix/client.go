package zabbix

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
	Timeout time.Duration

	// Se true, ignora validação TLS APENAS para chamadas do Zabbix
	InsecureTLS bool
}

func (c Client) Acknowledge(eventID string, message string) error {
	if strings.TrimSpace(c.BaseURL) == "" || strings.TrimSpace(c.Token) == "" || strings.TrimSpace(eventID) == "" {
		return nil
	}

	// timeout defensivo
	to := c.Timeout
	if to <= 0 {
		to = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), to)
	defer cancel()

	// ✅ eventids como array + auth no payload (compatível)
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  "event.acknowledge",
		"params": map[string]any{
			"eventids": []string{strings.TrimSpace(eventID)},
			"action":   6,
			"message":  message,
		},
		"auth": strings.TrimSpace(c.Token),
		"id":   1,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/") + "/api_jsonrpc.php"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Token))

	// ✅ cria um client só pro Zabbix (não afeta o resto do sistema)
	baseClient := c.HTTP
	if baseClient == nil {
		baseClient = &http.Client{}
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: c.InsecureTLS},
	}

	zclient := &http.Client{
		Timeout:   baseClient.Timeout,
		Transport: transport,
	}

	resp, err := zclient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Zabbix ack HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
