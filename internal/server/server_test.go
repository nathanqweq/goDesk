package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http/httptest"
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

	handleAlert("secret")(w, req)

	if w.Code != 401 {
		t.Fatalf("esperado 401, got %d", w.Code)
	}
}

func TestHandleAlertAcceptsCorrectToken(t *testing.T) {
	defer stubRunFn(func(cfg config.RuntimeConfig) error { return nil })()

	body, _ := json.Marshal(alertRequest{Domain: "https://example.com"})
	req := httptest.NewRequest("POST", "/alert", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()

	handleAlert("secret")(w, req)

	if w.Code != 200 {
		t.Fatalf("esperado 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleAlertRejectsInvalidJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/alert", bytes.NewBufferString(`{invalido`))
	w := httptest.NewRecorder()

	handleAlert("")(w, req)

	if w.Code != 400 {
		t.Fatalf("esperado 400, got %d", w.Code)
	}
}

func TestHandleAlertPropagatesRunError(t *testing.T) {
	defer stubRunFn(func(cfg config.RuntimeConfig) error { return errors.New("falha topdesk") })()

	req := httptest.NewRequest("POST", "/alert", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()

	handleAlert("")(w, req)

	if w.Code != 500 {
		t.Fatalf("esperado 500, got %d", w.Code)
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
