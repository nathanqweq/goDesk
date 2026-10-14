package zabbix

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type rpcReq struct {
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
}

func fakeServer(t *testing.T, handle func(r rpcReq) any) (*httptest.Server, *[]rpcReq) {
	t.Helper()
	var calls []rpcReq
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcReq
		_ = json.NewDecoder(r.Body).Decode(&req)
		calls = append(calls, req)
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "result": handle(req), "id": 1})
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestAddMessageDoesNotAcknowledge(t *testing.T) {
	srv, calls := fakeServer(t, func(r rpcReq) any { return map[string]any{"eventids": []string{"1"}} })
	c := Client{BaseURL: srv.URL, Token: "x"}

	if err := c.AddMessage("1", "oi"); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got := (*calls)[0].Params["action"]; got != float64(4) {
		t.Fatalf("esperava action=4 (só mensagem), veio %v", got)
	}

	if err := c.Acknowledge("1", "oi"); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got := (*calls)[1].Params["action"]; got != float64(6) {
		t.Fatalf("esperava action=6 (ack+mensagem), veio %v", got)
	}
}

func TestHostsByNameFallsBackToVisibleName(t *testing.T) {
	srv, calls := fakeServer(t, func(r rpcReq) any {
		filter := r.Params["filter"].(map[string]any)
		if _, ok := filter["name"]; ok {
			return []Host{{HostID: "30", Host: "sw02.local", Name: "Switch 02"}}
		}
		return []Host{{HostID: "20", Host: "SW-01", Name: "Switch 01"}}
	})
	c := Client{BaseURL: srv.URL, Token: "x"}

	got, err := c.HostsByName([]string{"sw-01", "Switch 02", "NAO-EXISTE"})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got["sw-01"].HostID != "20" || got["switch 02"].HostID != "30" {
		t.Fatalf("resolução errada: %+v", got)
	}
	if _, ok := got["nao-existe"]; ok {
		t.Fatal("host inexistente não deveria aparecer")
	}
	if len(*calls) != 2 {
		t.Fatalf("esperava 2 chamadas (host, name), veio %d", len(*calls))
	}
}

func TestCallReturnsRPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid params.","data":"No permissions"},"id":1}`))
	}))
	defer srv.Close()

	if _, err := (Client{BaseURL: srv.URL, Token: "x"}).ActiveProblems("10"); err == nil {
		t.Fatal("esperava erro RPC")
	}
}

func TestCallNotConfigured(t *testing.T) {
	if _, err := (Client{}).ActiveProblems("10"); err != ErrNotConfigured {
		t.Fatalf("esperava ErrNotConfigured, veio %v", err)
	}
}
