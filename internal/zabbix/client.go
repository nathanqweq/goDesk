package zabbix

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
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

// event.acknowledge action (bitmask)
const (
	ackActionAcknowledge = 2
	ackActionMessage     = 4
)

// ErrNotConfigured indica que GODESK_ZABBIX_URL/KEY não estão configurados.
var ErrNotConfigured = errors.New("zabbix não configurado (url/token vazios)")

// Acknowledge reconhece o evento e adiciona message (action=6) — encerra
// escalonamentos de Actions com condição "não reconhecido".
func (c Client) Acknowledge(eventID string, message string) error {
	return c.acknowledge(eventID, ackActionAcknowledge|ackActionMessage, message)
}

// AddMessage só comenta no evento (action=4), SEM reconhecer: o evento
// continua "não reconhecido" e um escalonamento da Action no Zabbix pode
// reenviar o alerta mais tarde (usado pela dependência de hosts, pra
// reavaliar quando o pai normalizar).
func (c Client) AddMessage(eventID string, message string) error {
	return c.acknowledge(eventID, ackActionMessage, message)
}

func (c Client) acknowledge(eventID string, action int, message string) error {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" || !c.configured() {
		log.Printf("[zabbix] ACK SKIP base_set=%v token_set=%v eventid=%q\n",
			strings.TrimSpace(c.BaseURL) != "", strings.TrimSpace(c.Token) != "", eventID)
		return nil
	}

	_, err := c.call("event.acknowledge", map[string]any{
		"eventids": []string{eventID},
		"action":   action,
		"message":  message,
	})
	if err != nil {
		return fmt.Errorf("Zabbix ack eventid=%s: %w", eventID, err)
	}

	log.Printf("[zabbix] ACK OK eventid=%q action=%d\n", eventID, action)
	return nil
}

// Host é o mínimo de host.get que o goDesk precisa.
type Host struct {
	HostID string `json:"hostid"`
	Host   string `json:"host"`
	Name   string `json:"name"`
}

// HostsByName resolve nomes de host pra hostid: primeiro pelo nome técnico
// ("host"), e o que não achar tenta pelo nome visível ("name"). Nomes sem
// correspondência simplesmente não aparecem no resultado.
func (c Client) HostsByName(names []string) (map[string]Host, error) {
	out := map[string]Host{}
	if len(names) == 0 {
		return out, nil
	}

	pending := names
	for _, field := range []string{"host", "name"} {
		if len(pending) == 0 {
			break
		}

		raw, err := c.call("host.get", map[string]any{
			"output": []string{"hostid", "host", "name"},
			"filter": map[string]any{field: pending},
		})
		if err != nil {
			return nil, err
		}

		var hosts []Host
		if err := json.Unmarshal(raw, &hosts); err != nil {
			return nil, fmt.Errorf("host.get: %w", err)
		}

		var next []string
		for _, n := range pending {
			found := false
			for _, h := range hosts {
				v := h.Host
				if field == "name" {
					v = h.Name
				}
				if strings.EqualFold(v, n) {
					out[strings.ToLower(n)] = h
					found = true
					break
				}
			}
			if !found {
				next = append(next, n)
			}
		}
		pending = next
	}

	return out, nil
}

// Problem é um problema ativo (problem.get).
type Problem struct {
	EventID  string `json:"eventid"`
	Name     string `json:"name"`
	Severity string `json:"severity"`
	Clock    string `json:"clock"`
}

// ActiveProblems devolve os problemas ativos (não resolvidos) do host,
// mais recentes primeiro.
func (c Client) ActiveProblems(hostID string) ([]Problem, error) {
	raw, err := c.call("problem.get", map[string]any{
		"output":    []string{"eventid", "name", "severity", "clock"},
		"hostids":   []string{hostID},
		"source":    0,
		"object":    0,
		"sortfield": []string{"eventid"},
		"sortorder": "DESC",
	})
	if err != nil {
		return nil, err
	}

	var problems []Problem
	if err := json.Unmarshal(raw, &problems); err != nil {
		return nil, fmt.Errorf("problem.get: %w", err)
	}
	return problems, nil
}

func (c Client) configured() bool {
	return strings.TrimSpace(c.BaseURL) != "" && strings.TrimSpace(c.Token) != ""
}

// call faz uma chamada JSON-RPC (token Bearer no header) e devolve o
// "result" cru, tratando erro HTTP e erro JSON-RPC (que vem com HTTP 200).
func (c Client) call(method string, params any) (json.RawMessage, error) {
	if !c.configured() {
		return nil, ErrNotConfigured
	}

	to := c.Timeout
	if to <= 0 {
		to = 10 * time.Second
	}

	url := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/") + "/api_jsonrpc.php"

	b, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), to)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Token))

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

	log.Printf("[zabbix] %s SEND url=%q insecureTLS=%v timeout=%s\n", method, url, c.InsecureTLS, to)

	resp, err := zclient.Do(req)
	if err != nil {
		log.Printf("[zabbix] %s ERROR http_do url=%q err=%v\n", method, url, err)
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	// limita body no log pra não explodir
	bodyLog := string(bodyBytes)
	if len(bodyLog) > 800 {
		bodyLog = bodyLog[:800] + "...(truncated)"
	}

	log.Printf("[zabbix] %s HTTP status=%d resp=%s\n", method, resp.StatusCode, bodyLog)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Zabbix %s HTTP %d: %s", method, resp.StatusCode, bodyLog)
	}

	// valida erro JSON-RPC mesmo com HTTP 200
	var rr rpcResp
	if err := json.Unmarshal(bodyBytes, &rr); err != nil {
		return nil, fmt.Errorf("Zabbix %s: resposta não é JSON-RPC válida: %v (body=%s)", method, err, bodyLog)
	}
	if rr.Error != nil {
		return nil, fmt.Errorf("Zabbix %s RPC error code=%d message=%q data=%q",
			method, rr.Error.Code, rr.Error.Message, rr.Error.Data)
	}

	return rr.Result, nil
}
