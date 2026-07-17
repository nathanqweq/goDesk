package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"godesk/internal/metrics"
	"godesk/internal/topdesk"
)

func TestCheckTopdeskOnceRecordsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	metricsFile := filepath.Join(t.TempDir(), "metrics.json")
	client := topdesk.Client{BaseURL: srv.URL, User: "u", Pass: "p", HTTP: srv.Client()}

	checkTopdeskOnce(client, metricsFile)

	snap, err := metrics.Read(metricsFile)
	if err != nil {
		t.Fatalf("erro ao ler métricas: %v", err)
	}
	if snap.TopdeskHealthChecksTotal != 1 {
		t.Fatalf("esperava TopdeskHealthChecksTotal=1, veio %d", snap.TopdeskHealthChecksTotal)
	}
	if snap.TopdeskHealthCheckErrors != 0 {
		t.Fatalf("esperava TopdeskHealthCheckErrors=0, veio %d", snap.TopdeskHealthCheckErrors)
	}
	if !snap.TopdeskHealthCheckLastOK {
		t.Fatal("esperava TopdeskHealthCheckLastOK=true")
	}
	if snap.TopdeskHealthCheckLastAt.IsZero() {
		t.Fatal("esperava TopdeskHealthCheckLastAt preenchido")
	}
}

func TestCheckTopdeskOnceRecordsFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	metricsFile := filepath.Join(t.TempDir(), "metrics.json")
	client := topdesk.Client{BaseURL: srv.URL, User: "u", Pass: "p", HTTP: srv.Client()}

	checkTopdeskOnce(client, metricsFile)

	snap, err := metrics.Read(metricsFile)
	if err != nil {
		t.Fatalf("erro ao ler métricas: %v", err)
	}
	if snap.TopdeskHealthChecksTotal != 1 {
		t.Fatalf("esperava TopdeskHealthChecksTotal=1, veio %d", snap.TopdeskHealthChecksTotal)
	}
	if snap.TopdeskHealthCheckErrors != 1 {
		t.Fatalf("esperava TopdeskHealthCheckErrors=1, veio %d", snap.TopdeskHealthCheckErrors)
	}
	if snap.TopdeskHealthCheckLastOK {
		t.Fatal("esperava TopdeskHealthCheckLastOK=false")
	}
	if snap.TopdeskHealthCheckLastError == "" {
		t.Fatal("esperava TopdeskHealthCheckLastError preenchido")
	}
}
