package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"godesk/internal/config"
)

func stubRunFn(fn func(config.RuntimeConfig) error) func() {
	orig := runFn
	runFn = fn
	return func() { runFn = orig }
}

func TestHandleAlertRequiresTokenWhenConfigured(t *testing.T) {
	req := httptest.NewRequest("POST", "/alert", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()

	handleAlert(config.ServiceConfig{Token: "secret"})(w, req)

	if w.Code != 401 {
		t.Fatalf("esperado 401, got %d", w.Code)
	}
}

func TestHandleAlertAcceptsCorrectToken(t *testing.T) {
	defer stubRunFn(func(cfg config.RuntimeConfig) error { return nil })()

	body, _ := json.Marshal(alertRequest{TicketName: "chamado-x", RawData: `{}`})
	req := httptest.NewRequest("POST", "/alert", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()

	handleAlert(config.ServiceConfig{Token: "secret"})(w, req)

	if w.Code != 200 {
		t.Fatalf("esperado 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleAlertRejectsInvalidJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/alert", bytes.NewBufferString(`{invalido`))
	w := httptest.NewRecorder()

	handleAlert(config.ServiceConfig{})(w, req)

	if w.Code != 400 {
		t.Fatalf("esperado 400, got %d", w.Code)
	}
}

func TestHandleAlertPropagatesRunError(t *testing.T) {
	defer stubRunFn(func(cfg config.RuntimeConfig) error { return errors.New("falha topdesk") })()

	req := httptest.NewRequest("POST", "/alert", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()

	handleAlert(config.ServiceConfig{})(w, req)

	if w.Code != 500 {
		t.Fatalf("esperado 500, got %d", w.Code)
	}
}

func TestHandleConfigRequiresTokenWhenConfigured(t *testing.T) {
	req := httptest.NewRequest("POST", "/config", bytes.NewBufferString("default:\n  urgency: Baixa\n"))
	w := httptest.NewRecorder()

	handleConfig("secret", filepath.Join(t.TempDir(), "godesk-config.yaml"))(w, req)

	if w.Code != 401 {
		t.Fatalf("esperado 401, got %d", w.Code)
	}
}

func TestHandleConfigRejectsInvalidYAML(t *testing.T) {
	target := filepath.Join(t.TempDir(), "godesk-config.yaml")
	req := httptest.NewRequest("POST", "/config", bytes.NewBufferString("::: nao é yaml"))
	w := httptest.NewRecorder()

	handleConfig("", target)(w, req)

	if w.Code != 400 {
		t.Fatalf("esperado 400, got %d: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("arquivo não deveria ter sido criado para YAML inválido")
	}
}

func TestHandleConfigWritesValidYAML(t *testing.T) {
	target := filepath.Join(t.TempDir(), "godesk-config.yaml")
	yamlBody := "default:\n  urgency: Alta\n  impact: Alto\n"
	req := httptest.NewRequest("POST", "/config", bytes.NewBufferString(yamlBody))
	w := httptest.NewRecorder()

	handleConfig("", target)(w, req)

	if w.Code != 200 {
		t.Fatalf("esperado 200, got %d: %s", w.Code, w.Body.String())
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("arquivo não foi escrito: %v", err)
	}
	if string(got) != yamlBody {
		t.Fatalf("conteúdo gravado difere do enviado: %q", got)
	}
}

func TestHandleConfigCreatesBackupOfPreviousFile(t *testing.T) {
	target := filepath.Join(t.TempDir(), "godesk-config.yaml")
	if err := os.WriteFile(target, []byte("default:\n  urgency: Baixa\n"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	req := httptest.NewRequest("POST", "/config", bytes.NewBufferString("default:\n  urgency: Alta\n"))
	w := httptest.NewRecorder()

	handleConfig("", target)(w, req)

	if w.Code != 200 {
		t.Fatalf("esperado 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("resposta inválida: %v", err)
	}
	if resp["backup"] == "" {
		t.Fatalf("esperava caminho de backup na resposta, veio vazio")
	}
	if _, err := os.Stat(resp["backup"]); err != nil {
		t.Fatalf("backup não foi criado em %q: %v", resp["backup"], err)
	}
}

func TestHandleHealth(t *testing.T) {
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()

	handleHealth(w, req)

	if w.Code != 200 {
		t.Fatalf("esperado 200, got %d", w.Code)
	}
}
