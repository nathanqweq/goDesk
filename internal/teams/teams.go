package teams

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Cores do container de título — mesmas do media type "MS Teams Workflow"
// do Zabbix (onProblem/onResolve).
const (
	StyleProblem = "attention"
	StyleResolve = "good"
)

// Message é o conteúdo de um card: Subject vira o título em destaque (com a
// cor Style), cada item de Lines vira um TextBlock, e EventURL (se não
// vazio) vira o botão "Event info".
type Message struct {
	Subject  string
	Lines    []string
	Style    string
	EventURL string
}

// ProblemURL monta o link do evento no frontend do Zabbix, igual ao
// createProblemURL do media type (event_source 0 = trigger). Retorna vazio
// se zabbixURL não estiver configurado.
func ProblemURL(zabbixURL, triggerID, eventID string) string {
	zabbixURL = strings.TrimRight(strings.TrimSpace(zabbixURL), "/")
	if zabbixURL == "" {
		return ""
	}
	if strings.TrimSpace(triggerID) == "" || strings.TrimSpace(eventID) == "" {
		return zabbixURL
	}
	return zabbixURL + "/tr_events.php?triggerid=" + url.QueryEscape(triggerID) + "&eventid=" + url.QueryEscape(eventID)
}

// Body monta o payload do Workflow do Teams ("Post a message in a channel
// when a webhook request is received"): uma mensagem com um Adaptive Card.
func Body(m Message) map[string]any {
	items := []any{
		map[string]any{
			"type":  "Container",
			"style": m.Style,
			"bleed": true,
			"items": []any{
				map[string]any{
					"type":   "TextBlock",
					"size":   "Medium",
					"wrap":   true,
					"weight": "Bolder",
					"text":   m.Subject,
				},
			},
		},
	}
	for _, line := range m.Lines {
		items = append(items, map[string]any{
			"type": "TextBlock",
			"wrap": true,
			"text": line,
		})
	}

	content := map[string]any{
		"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
		"type":    "AdaptiveCard",
		"version": "1.4",
		"body":    items,
	}
	if m.EventURL != "" {
		content["actions"] = []any{
			map[string]any{
				"type":  "Action.OpenUrl",
				"title": "Event info",
				"url":   m.EventURL,
			},
		}
	}

	return map[string]any{
		"type": "message",
		"attachments": []any{
			map[string]any{
				"contentType": "application/vnd.microsoft.card.adaptive",
				"contentUrl":  nil,
				"content":     content,
			},
		},
	}
}

// Send faz o POST do card no webhook do Workflow. Qualquer status fora de
// 2xx é erro (o Workflow costuma responder 202 Accepted).
func Send(httpClient *http.Client, webhook string, m Message) error {
	webhook = strings.TrimSpace(webhook)
	if webhook == "" {
		return fmt.Errorf("teams: webhook vazio")
	}
	if !strings.HasPrefix(strings.ToLower(webhook), "https://") && !strings.HasPrefix(strings.ToLower(webhook), "http://") {
		return fmt.Errorf("teams: webhook precisa começar com http(s)://: %q", webhook)
	}

	b, err := json.Marshal(Body(m))
	if err != nil {
		return err
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	req, err := http.NewRequest(http.MethodPost, webhook, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 800))
		return fmt.Errorf("teams: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
