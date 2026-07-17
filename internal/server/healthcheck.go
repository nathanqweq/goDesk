package server

import (
	"context"
	"log"
	"net/http"
	"time"

	"godesk/internal/config"
	"godesk/internal/metrics"
	"godesk/internal/topdesk"
)

// runTopdeskHealthCheck testa a API do TopDesk periodicamente
// (GET /tas/api/incidents?pageSize=1), numa goroutine própria — nunca
// compete com o processamento de /alert (nenhum lock compartilhado). Para
// de rodar quando ctx é cancelado (shutdown do servidor).
func runTopdeskHealthCheck(ctx context.Context, opts config.ServiceConfig) {
	client := topdesk.Client{
		BaseURL: opts.TopdeskDomain,
		User:    opts.TopdeskUser,
		Pass:    opts.TopdeskPass,
		HTTP:    &http.Client{Timeout: 15 * time.Second},
	}

	log.Printf("[healthcheck] ativado, intervalo=%s domain=%q\n", opts.HealthCheckInterval, opts.TopdeskDomain)

	metricsFile := config.MetricsFileFromEnv()

	ticker := time.NewTicker(opts.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[healthcheck] encerrando (shutdown do servidor)")
			return
		case <-ticker.C:
			checkTopdeskOnce(client, metricsFile)
		}
	}
}

func checkTopdeskOnce(client topdesk.Client, metricsFile string) {
	status, elapsed, err := client.HealthCheck()

	metrics.RecordAndLog(metricsFile, func(m *metrics.Snapshot) {
		m.TopdeskHealthChecksTotal++
		m.TopdeskHealthCheckLastAt = time.Now().UTC()
		m.TopdeskHealthCheckLastLatencyMs = elapsed.Milliseconds()
		if err != nil {
			m.TopdeskHealthCheckErrors++
			m.TopdeskHealthCheckLastOK = false
			m.TopdeskHealthCheckLastError = err.Error()
		} else {
			m.TopdeskHealthCheckLastOK = true
			m.TopdeskHealthCheckLastError = ""
		}
	})

	if err != nil {
		log.Printf("[healthcheck] FALHA status=%d latencia=%s erro=%v\n", status, elapsed, err)
	} else {
		log.Printf("[healthcheck] ok status=%d latencia=%s\n", status, elapsed)
	}
}
