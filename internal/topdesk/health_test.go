package topdesk

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthCheckSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tas/api/incidents" {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		if r.URL.Query().Get("pageSize") != "1" {
			t.Errorf("esperava pageSize=1, veio %q", r.URL.Query().Get("pageSize"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := Client{BaseURL: srv.URL, User: "u", Pass: "p", HTTP: srv.Client()}

	status, elapsed, err := c.HealthCheck()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("esperava status 200, veio %d", status)
	}
	if elapsed < 0 {
		t.Fatalf("latência inválida: %s", elapsed)
	}
}

func TestHealthCheckHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := Client{BaseURL: srv.URL, User: "u", Pass: "p", HTTP: srv.Client()}

	status, _, err := c.HealthCheck()
	if err == nil {
		t.Fatal("esperava erro para status 401")
	}
	if status != http.StatusUnauthorized {
		t.Fatalf("esperava status 401, veio %d", status)
	}
}
