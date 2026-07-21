package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleConfigTestRequiresTokenWhenConfigured(t *testing.T) {
	req := httptest.NewRequest("POST", "/config/test", strings.NewReader("default: {}"))
	w := httptest.NewRecorder()

	handleConfigTest("secret", filepath.Join(t.TempDir(), "godesk-config.yaml"))(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("esperado 401, got %d", w.Code)
	}
}

func TestHandleConfigTestRejectsInvalidYAML(t *testing.T) {
	req := httptest.NewRequest("POST", "/config/test", strings.NewReader(": : : nao é yaml válido"))
	w := httptest.NewRecorder()

	handleConfigTest("", filepath.Join(t.TempDir(), "godesk-config.yaml"))(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleConfigTestDoesNotModifyRealFile(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "godesk-config.yaml")

	if err := os.WriteFile(configFile, []byte("conteudo original"), 0644); err != nil {
		t.Fatalf("erro ao preparar arquivo: %v", err)
	}

	req := httptest.NewRequest("POST", "/config/test", strings.NewReader("default:\n  urgency: Alta\nclients: {}\n"))
	w := httptest.NewRecorder()

	handleConfigTest("", configFile)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d: %s", w.Code, w.Body.String())
	}

	got, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("erro ao reler arquivo: %v", err)
	}
	if string(got) != "conteudo original" {
		t.Fatalf("handleConfigTest não deveria alterar o arquivo real, veio %q", got)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("erro ao listar dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("esperava só o arquivo original no dir (sem sobras de arquivo temporário), achei: %v", entries)
	}
}
