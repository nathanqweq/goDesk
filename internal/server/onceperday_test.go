package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"godesk/internal/app"
)

func TestHandleOncePerDayListRequiresTokenWhenConfigured(t *testing.T) {
	req := httptest.NewRequest("GET", "/once-per-day", nil)
	w := httptest.NewRecorder()

	handleOncePerDayList("secret")(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("esperado 401, got %d", w.Code)
	}
}

func TestHandleOncePerDayListReturnsEntries(t *testing.T) {
	req := httptest.NewRequest("GET", "/once-per-day", nil)
	w := httptest.NewRecorder()

	handleOncePerDayList("")(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		Entries []app.OncePerDayEntry `json:"entries"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("resposta inválida: %v", err)
	}
}
