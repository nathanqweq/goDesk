package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"godesk/internal/app"
	"godesk/internal/config"
)

// runFn permite substituir app.Run em testes.
var runFn = app.Run

// alertRequest só carrega o que de fato muda por alerta — TopDesk e
// Zabbix (domain/user/pass/url/key) vêm do ServiceConfig do processo,
// carregado uma vez em Run (ver config.ServiceConfigFromEnv).
type alertRequest struct {
	TicketName string `json:"ticket_name"`
	RawData    string `json:"rawdata"`
}

// Run sobe o servidor HTTP do goDesk e bloqueia até receber SIGINT/SIGTERM,
// encerrando graciosamente em seguida (compatível com `systemctl stop`).
func Run(opts config.ServiceConfig) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("POST /alert", handleAlert(opts))
	mux.HandleFunc("POST /config", handleConfig(opts.Token, opts.ConfigFile))
	mux.HandleFunc("POST /config/test", handleConfigTest(opts.Token, opts.ConfigFile))
	mux.HandleFunc("POST /validate-client", handleValidateClient(opts))
	mux.HandleFunc("GET /once-per-day", handleOncePerDayList(opts.Token))

	srv := &http.Server{
		Addr:         opts.ListenAddr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if opts.HealthCheckInterval > 0 && opts.TopdeskDomain != "" {
		go runTopdeskHealthCheck(ctx, opts)
	} else {
		log.Println("[healthcheck] desabilitado (GODESK_HEALTHCHECK_INTERVAL ou GODESK_TOPDESK_DOMAIN não configurados)")
	}

	go runOncePerDayPurge(ctx)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("[server] ouvindo em %s\n", opts.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Println("[server] sinal de encerramento recebido, finalizando...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func handleAlert(opts config.ServiceConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, opts.Token) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		var req alertRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "json inválido: " + err.Error()})
			return
		}

		cfg := config.FromValues(opts, req.TicketName, req.RawData)

		if err := runFn(cfg); err != nil {
			log.Printf("[server] ERRO ao processar alerta: %v\n", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func handleConfig(token, configFile string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, token) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "falha ao ler corpo: " + err.Error()})
			return
		}

		if _, err := config.ParsePolicies(body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "yaml inválido: " + err.Error()})
			return
		}

		backup, err := config.SaveFile(configFile, body)
		if err != nil {
			log.Printf("[server] ERRO ao gravar config (%s): %v\n", configFile, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		log.Printf("[server] config atualizada (%s, backup=%q)\n", configFile, backup)
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "backup": backup})
	}
}

func authorized(r *http.Request, token string) bool {
	if token == "" {
		return true
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return got != "" && got == token
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
