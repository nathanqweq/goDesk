package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"godesk/internal/flowlog"
)

func TestHandleFlowsHistory(t *testing.T) {
	p := filepath.Join(t.TempDir(), "h.jsonl")
	for _, e := range []flowlog.Entry{{EventID: "1", Rule: "A", Flow: "infra"}, {EventID: "2", Rule: "B", Flow: "infra"}} {
		if err := flowlog.Append(p, e); err != nil {
			t.Fatal(err)
		}
	}
	h := handleFlowsHistory("tok", p)

	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/flows/history", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("sem token: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/flows/history?rule=b", nil)
	req.Header.Set("Authorization", "Bearer tok")
	h(rec, req)
	var out struct {
		Entries []flowlog.Entry `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || rec.Code != 200 {
		t.Fatalf("resposta %d %s", rec.Code, rec.Body.String())
	}
	if len(out.Entries) != 1 || out.Entries[0].EventID != "2" {
		t.Errorf("filtro rule=b: %+v", out.Entries)
	}
}
