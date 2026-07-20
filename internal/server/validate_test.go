package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"godesk/internal/config"
)

func TestHandleValidateClientRequiresTokenWhenConfigured(t *testing.T) {
	req := httptest.NewRequest("POST", "/validate-client", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()

	handleValidateClient(config.ServiceConfig{Token: "secret"})(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("esperado 401, got %d", w.Code)
	}
}

func TestHandleValidateClientChecksOnlyFilledFields(t *testing.T) {
	topdesk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tas/api/operators/id/op-valido":
			w.WriteHeader(http.StatusOK)
		case "/tas/api/operatorgroups/id/grp-invalido":
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("chamada inesperada: %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer topdesk.Close()

	opts := config.ServiceConfig{TopdeskDomain: topdesk.URL, TopdeskUser: "u", TopdeskPass: "p"}

	body, _ := json.Marshal(map[string]string{"operator": "op-valido", "oper_group": "grp-invalido"})
	req := httptest.NewRequest("POST", "/validate-client", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handleValidateClient(opts)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]fieldResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("resposta inválida: %v", err)
	}

	if !result["operator"].Valid {
		t.Fatalf("esperava operator válido, veio %+v", result["operator"])
	}
	if result["oper_group"].Valid {
		t.Fatalf("esperava oper_group inválido, veio %+v", result["oper_group"])
	}
	if _, hasSla := result["sla"]; hasSla {
		t.Fatal("não deveria ter resultado pra campo não enviado (sla)")
	}
}
