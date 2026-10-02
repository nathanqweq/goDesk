package teams

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendPostsAdaptiveCard(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("esperava Content-Type application/json, veio %q", ct)
		}
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &got); err != nil {
			t.Errorf("body não é JSON: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	err := Send(srv.Client(), srv.URL, Message{
		Subject:  "Problema: ICMP down",
		Style:    StyleProblem,
		Lines:    []string{"Host: SW-01", "Chamado TopDesk: I2610-001"},
		EventURL: ProblemURL("https://zabbix.exemplo/", "123", "456"),
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	att := got["attachments"].([]any)[0].(map[string]any)
	content := att["content"].(map[string]any)
	body := content["body"].([]any)
	if len(body) != 3 {
		t.Fatalf("esperava título + 2 linhas, veio %d itens", len(body))
	}
	title := body[0].(map[string]any)
	if title["style"] != StyleProblem {
		t.Fatalf("esperava style=%q, veio %v", StyleProblem, title["style"])
	}
	if txt := body[2].(map[string]any)["text"]; txt != "Chamado TopDesk: I2610-001" {
		t.Fatalf("linha do chamado errada: %v", txt)
	}
	action := content["actions"].([]any)[0].(map[string]any)
	if action["url"] != "https://zabbix.exemplo/tr_events.php?triggerid=123&eventid=456" {
		t.Fatalf("url do evento errada: %v", action["url"])
	}
}

func TestSendReturnsErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "workflow desativado", http.StatusBadRequest)
	}))
	defer srv.Close()

	err := Send(srv.Client(), srv.URL, Message{Subject: "x"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 400") {
		t.Fatalf("esperava erro HTTP 400, veio %v", err)
	}
}

func TestBodyOmitsActionsWithoutEventURL(t *testing.T) {
	content := Body(Message{Subject: "x"})["attachments"].([]any)[0].(map[string]any)["content"].(map[string]any)
	if _, ok := content["actions"]; ok {
		t.Fatal("sem EventURL não deveria ter botão Event info")
	}
}

func TestProblemURLEmptyWithoutZabbixURL(t *testing.T) {
	if u := ProblemURL("  ", "1", "2"); u != "" {
		t.Fatalf("esperava vazio, veio %q", u)
	}
}
