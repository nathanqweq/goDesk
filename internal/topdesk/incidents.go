package topdesk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (c Client) TicketExists(briefDescription string) (exists bool, ticketID string, status string, err error) {
	base := strings.TrimRight(c.BaseURL, "/")
	q := fmt.Sprintf(`processingStatus.name!=Fechado;briefDescription=="%s"`, escapeQuery(ticketName))

	// URL encode do query inteiro
	url := base + "/tas/api/incidents?query=" + url.QueryEscape(q)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.authHeader())

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return false, "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 204 {
		return false, "", "", nil
	}

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, "", "", fmt.Errorf("TopDesk TicketExists HTTP %d: %s", resp.StatusCode, string(body))
	}

	incidents, err := parseIncidentList(body)
	if err != nil {
		return false, "", "", fmt.Errorf("TopDesk TicketExists JSON: %w body=%s", err, string(body))
	}
	if len(incidents) == 0 {
		return false, "", "", nil
	}
	return true, incidents[0].Number, incidents[0].ProcessingStatus.Name, nil
}

func (c Client) CreateTicket(payload any) (string, error) {
	url := strings.TrimRight(c.BaseURL, "/") + "/tas/api/incidents"
	b, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.authHeader())

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("TopDesk CreateTicket HTTP %d: %s", resp.StatusCode, string(body))
	}

	var inc Incident
	if err := json.Unmarshal(body, &inc); err != nil {
		return "", fmt.Errorf("TopDesk CreateTicket JSON: %w body=%s", err, string(body))
	}
	if strings.TrimSpace(inc.Number) == "" {
		return "", fmt.Errorf("TopDesk CreateTicket: number vazio body=%s", string(body))
	}
	return inc.Number, nil
}

func (c Client) PatchTicket(ticketID string, payload any) error {
	url := strings.TrimRight(c.BaseURL, "/") + "/tas/api/incidents/number/" + ticketID
	b, _ := json.Marshal(payload)

	req, _ := http.NewRequest("PATCH", url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.authHeader())

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("TopDesk PatchTicket HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func escapeQuery(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}

func parseIncidentList(body []byte) ([]Incident, error) {
	var arr []Incident
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr, nil
	}

	var single Incident
	if err := json.Unmarshal(body, &single); err == nil {
		if single.Number != "" || single.ProcessingStatus.Name != "" {
			return []Incident{single}, nil
		}
	}

	var wrapped struct {
		Results []Incident `json:"results"`
		Data    []Incident `json:"data"`
		Items   []Incident `json:"items"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil {
		switch {
		case len(wrapped.Results) > 0:
			return wrapped.Results, nil
		case len(wrapped.Data) > 0:
			return wrapped.Data, nil
		case len(wrapped.Items) > 0:
			return wrapped.Items, nil
		}
		return nil, nil
	}

	return nil, fmt.Errorf("unexpected response format")
}
