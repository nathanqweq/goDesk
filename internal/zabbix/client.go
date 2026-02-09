package zabbix

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcErr         `json:"error"`
	ID      any             `json:"id"`
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
		log.Printf("[zabbix] ACK SKIP base_set=%v token_set=%v eventid=%q\n",
			base != "", token != "", eventID)
		return nil
	}

	to := c.Timeout
	if to <= 0 {
		to = 10 * time.Second
	}

	url := strings.TrimRight(base, "/") + "/api_jsonrpc.php"

	// payload JSON-RPC (Bearer no header)
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  "event.acknowledge",
		"params": map[string]any{
			"eventids": []string{eventID},
			"action":   6,
			"message":  message,
		},
		"id": 1,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[zabbix] ACK ERROR marshal eventid=%q err=%v\n", eventID, err)
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), to)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		log.Printf("[zabbix] ACK ERROR build_request eventid=%q err=%v\n", eventID, err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	// client só pro Zabbix (não afeta TopDesk)
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

	log.Printf("[zabbix] ACK SEND url=%q insecureTLS=%v eventid=%q msg_len=%d timeout=%s\n",
		url, c.InsecureTLS, eventID, len(message), to)

	resp, err := zclient.Do(req)
	if err != nil {
		log.Printf("[zabbix] ACK ERROR http_do url=%q eventid=%q err=%v\n", url, eventID, err)
		return err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	body := string(bodyBytes)

	// limita body no log pra não explodir
	bodyLog := body
	if len(bodyLog) > 800 {
		bodyLog = bodyLog[:800] + "...(truncated)"
	}

	log.Printf("[zabbix] ACK HTTP status=%d eventid=%q resp=%s\n", resp.StatusCode, eventID, bodyLog)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Zabbix ack HTTP %d: %s", resp.StatusCode, body)
	}

	// valida erro JSON-RPC mesmo com HTTP 200
	var rr rpcResp
	if err := json.Unmarshal(bodyBytes, &rr); err != nil {
		// não dá pra garantir que aplicou
		return fmt.Errorf("Zabbix ack: resposta não é JSON-RPC válida: %v (body=%s)", err, bodyLog)
	}
	if rr.Error != nil {
		return fmt.Errorf("Zabbix ack RPC error code=%d message=%q data=%q",
			rr.Error.Code, rr.Error.Message, rr.Error.Data)
	}

	log.Printf("[zabbix] ACK OK eventid=%q\n", eventID)
	return nil
}
