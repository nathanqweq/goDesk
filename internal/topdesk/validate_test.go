package topdesk

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIDExistsFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tas/api/operators/id/abc-123" {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"abc-123"}`))
	}))
	defer srv.Close()

	c := Client{BaseURL: srv.URL, User: "u", Pass: "p", HTTP: srv.Client()}

	exists, err := c.OperatorExists("abc-123")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !exists {
		t.Fatal("esperava exists=true")
	}
}

func TestIDExistsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := Client{BaseURL: srv.URL, User: "u", Pass: "p", HTTP: srv.Client()}

	exists, err := c.OperatorGroupExists("nao-existe")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if exists {
		t.Fatal("esperava exists=false")
	}
}

func TestIDExistsUnexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := Client{BaseURL: srv.URL, User: "u", Pass: "p", HTTP: srv.Client()}

	_, err := c.OperatorExists("qualquer")
	if err == nil {
		t.Fatal("esperava erro para status 500")
	}
}
