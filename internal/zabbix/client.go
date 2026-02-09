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

type rpcResp struct {
	JSONRPC string   `json:"jsonrpc"`
	Result  any      `json:"result"`
	Error   *rpcErr  `json:"error"`
	ID      any      `json:"id"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

func (c Client) Acknowledge(eventID string, message string) error {
	base := strings.TrimSpace(c.BaseURL)
	token := strings.TrimSpace(c.Token)
	eventID = strings.TrimSpace(eventID)

	if base == "" || token == "" || eventID == "" {
		return nil
	}

	to := c.Timeout
	if to <= 0 {
		to = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), to)
	defer cancel()

	// ✅ Bearer via header (sem "auth" no payload)
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  "event.acknowledge",
		"params": map[string]any{
			// pode ser string ou array; mantive array que é mais compatível
			"eventids": []string{eventID},
			"action":   6,
			"message":  message,
		},
		"id": 1,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := strings.TrimRight(base, "/") + "/api_jsonrpc.php"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	// ✅ client só pro Zabbix
	baseClient := c.HTTP
	if baseClient == nil {
		baseClient = &http.Client{}
	}

	zclient := &http.Client{
		Timeout: baseClient.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: c.InsecureTLS},
		},
	}

	resp, err := zclient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// HTTP error
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Zabbix ack HTTP %d: %s", resp.StatusCode, string(body))
	}

	// ✅ se vier erro JSON-RPC com HTTP 200, pega também
	var rr rpcResp
	if err := json.Unmarshal(body, &rr); err == nil && rr.Error != nil {
		return fmt.Errorf("Zabbix ack RPC error code=%d message=%q data=%q",
			rr.Error.Code, rr.Error.Message, rr.Error.Data)
	}

	return nil
}
