package zabbix

import (
	"bytes"
	"context"
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
}

func (c Client) Acknowledge(eventID string, message string) error {
	if strings.TrimSpace(c.BaseURL) == "" || strings.TrimSpace(c.Token) == "" || strings.TrimSpace(eventID) == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  "event.acknowledge",
		"params": map[string]any{
			"eventids": eventID,
			"action":   6,
			"message":  message,
		},
		"id": 1,
	}

	b, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(c.BaseURL, "/")+"/api_jsonrpc.php", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTP.Do(req)
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
