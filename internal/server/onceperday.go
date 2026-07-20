package server

import (
	"context"
	"net/http"
	"time"

	"godesk/internal/app"
)

// handleOncePerDayList expõe a lista "um por dia" em memória (ver
// internal/app/onceperday.go) — usado pelo comando `godesk --once-per-day`
// pra conferir se a deduplicação de e-mail está funcionando.
func handleOncePerDayList(token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, token) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"entries": app.OncePerDayList()})
	}
}

// runOncePerDayPurge remove periodicamente entradas de dias anteriores da
// lista "um por dia" — só higiene de memória (a checagem de duplicidade já
// ignora entradas de dias passados por conta própria). Numa goroutine
// própria, mesmo esqueleto de runTopdeskHealthCheck.
func runOncePerDayPurge(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			app.OncePerDayPurge()
		}
	}
}
