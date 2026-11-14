package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"godesk/internal/config"
)

const flowTestFlows = `
calendars:
  comercial: {days: [seg, ter, qua, qui, sex], start: "08:00", end: "18:00"}
flows:
  - name: infra
    nodes:
      - {id: inicio, type: start}
      - {id: horario, type: if, conditions: [{field: calendar.comercial, op: is_false}]}
      - {id: fora, type: set, set: [{field: instrucao, value: "Fora do horário: LIGAR para o cliente"}, {field: urgency, value: Alta}]}
      - {id: crit, type: switch, field: host.tag.Criticidade, cases: [{value: Critico}]}
      - {id: ligar, type: set, set: [{field: instrucao, value: "Host crítico ({host.name}): LIGAR para o cliente"}, {field: urgency, value: Alta}, {field: oper_group, value: "{host.tag.Grupo}"}]}
      - {id: info, type: if, conditions: [{field: severity.level, op: lte, value: "1"}]}
      - {id: nao, type: end, action: skip, comment: "goDesk: {trigger} informativo, sem chamado"}
    links:
      - {from: inicio, to: horario}
      - {from: horario, port: "true", to: fora}
      - {from: horario, port: "false", to: crit}
      - {from: crit, port: Critico, to: ligar}
      - {from: crit, port: default, to: info}
      - {from: info, port: "true", to: nao}
`

func flowTestPolicies(flow, mode string) string {
	return `
default:
  urgency: Baixa
  impact: Sem impacto
  topdesk: {contract: C1, operator: OP1, oper_group: GRUPO_RULE, main_caller: a@b.c}
rules:
  infra:
    urgency: Média
    flow: "` + flow + `"
    flow_mode: "` + mode + `"
`
}

// fakeBackend responde como TopDesk (/tas/api/...) e Zabbix
// (/api_jsonrpc.php) e grava o que recebeu.
type fakeBackend struct {
	mu       sync.Mutex
	created  []map[string]any
	patches  []map[string]any
	zbxCalls []string
	acks     []string
	hostTags []map[string]string
}

func (f *fakeBackend) handler(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	body, _ := io.ReadAll(r.Body)

	switch {
	case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/tas/api/incidents"):
		w.WriteHeader(http.StatusNoContent)
	case r.Method == "POST" && r.URL.Path == "/tas/api/incidents":
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		f.created = append(f.created, m)
		_, _ = w.Write([]byte(`{"number":"I2611-001"}`))
	case r.Method == "PATCH":
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		f.patches = append(f.patches, m)
		_, _ = w.Write([]byte(`{}`))
	case r.URL.Path == "/api_jsonrpc.php":
		var req struct {
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		f.zbxCalls = append(f.zbxCalls, req.Method)
		var result any = []any{}
		switch req.Method {
		case "host.get":
			tags := []any{}
			for _, t := range f.hostTags {
				tags = append(tags, map[string]any{"tag": t["tag"], "value": t["value"]})
			}
			result = []any{map[string]any{
				"hostid": "10500", "host": "SW_CORE_UHCC_1", "name": "Switch Core UHCC 1",
				"tags": tags, "hostgroups": []any{map[string]any{"name": "POWERPLANT/GERACAO/UHCC"}},
			}}
		case "event.get":
			result = []any{map[string]any{"eventid": "1", "tags": []any{}}}
		case "event.acknowledge":
			if m, ok := req.Params["message"].(string); ok {
				f.acks = append(f.acks, m)
			}
			result = map[string]any{"eventids": []string{"1"}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "result": result, "id": 1})
	default:
		http.NotFound(w, r)
	}
}

func runFlowAlert(t *testing.T, flow, mode, severity string, hostTags []map[string]string, now string) *fakeBackend {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "godesk-config.yaml")
	flowsPath := filepath.Join(dir, "godesk-flows.yaml")
	if err := os.WriteFile(cfgPath, []byte(flowTestPolicies(flow, mode)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(flowsPath, []byte(flowTestFlows), 0o644); err != nil {
		t.Fatal(err)
	}

	fb := &fakeBackend{hostTags: hostTags}
	srv := httptest.NewServer(http.HandlerFunc(fb.handler))
	defer srv.Close()

	when, err := time.ParseInLocation("2006-01-02 15:04", now, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	prev := flowNow
	flowNow = func() time.Time { return when }
	defer func() { flowNow = prev }()

	raw, _ := json.Marshal(map[string]string{
		"status": "PROBLEM", "host": "SW_CORE_UHCC_1", "host_id": "10500",
		"trigger": "Unavailable by ICMP ping", "severity": severity,
		"event_id": "1", "trigger_id": "2", "event_value": "1", "rule_name": "infra",
		"date": "2026.11.16", "hour": "10:00:00",
	})

	err = Run(config.RuntimeConfig{
		Domain: srv.URL, User: "u", Pass: "p",
		TicketName: "[UHCC]: Unavailable by ICMP ping - SW_CORE_UHCC_1",
		RawData:    string(raw),
		ZabbixURL:  srv.URL, ZabbixKey: "k",
		ConfigFile: cfgPath, FlowsFile: flowsPath,
		MetricsFile: filepath.Join(dir, "metrics.json"),
		TimeoutSec:  5,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return fb
}

func field(m map[string]any, key, sub string) string {
	v, _ := m[key].(map[string]any)
	s, _ := v[sub].(string)
	return s
}

func TestRunFluxoHostCriticoDefineCamposEInstrucao(t *testing.T) {
	fb := runFlowAlert(t, "infra", "", "High",
		[]map[string]string{{"tag": "Criticidade", "value": "Critico"}, {"tag": "Grupo", "value": "GRUPO_CRIT"}},
		"2026-11-16 10:00")

	if len(fb.created) != 1 {
		t.Fatalf("esperado 1 chamado, veio %d", len(fb.created))
	}
	c := fb.created[0]
	if got := field(c, "urgency", "name"); got != "Alta" {
		t.Errorf("urgency = %q, esperado Alta (fluxo sobrescreve a rule)", got)
	}
	if got := field(c, "operatorGroup", "id"); got != "GRUPO_CRIT" {
		t.Errorf("operatorGroup = %q, esperado GRUPO_CRIT (template {host.tag.Grupo})", got)
	}

	found := false
	for _, p := range fb.patches {
		a, _ := p["action"].(string)
		if strings.Contains(a, "Instrução:") {
			found = true
			if !strings.Contains(a, "Host crítico (Switch Core UHCC 1): LIGAR para o cliente") {
				t.Errorf("instrução = %q", a)
			}
			if inv, _ := p["actionInvisibleForCaller"].(bool); !inv {
				t.Error("instrução precisa ser invisível pro solicitante")
			}
		}
	}
	if !found {
		t.Errorf("instrução não registrada no chamado; patches=%v", fb.patches)
	}
}

func TestRunFluxoInformativoNaoAbre(t *testing.T) {
	fb := runFlowAlert(t, "infra", "on", "Information", nil, "2026-11-16 10:00")
	if len(fb.created) != 0 {
		t.Fatalf("não devia abrir chamado, abriu %d", len(fb.created))
	}
	if len(fb.acks) != 1 || !strings.Contains(fb.acks[0], "Unavailable by ICMP ping informativo") {
		t.Errorf("comentário no evento = %v", fb.acks)
	}
}

func TestRunFluxoForaDoHorario(t *testing.T) {
	// sábado: não precisa nem consultar tags do host
	fb := runFlowAlert(t, "infra", "", "High", nil, "2026-11-14 22:00")
	if len(fb.created) != 1 || field(fb.created[0], "urgency", "name") != "Alta" {
		t.Fatalf("esperado chamado com urgência Alta: %v", fb.created)
	}
	for _, m := range fb.zbxCalls {
		if m == "host.get" {
			t.Errorf("host.get não devia ser chamado (fluxo não chegou no switch de tag)")
		}
	}
}

func TestRunFluxoShadowNaoAltera(t *testing.T) {
	fb := runFlowAlert(t, "infra", "shadow", "Information", nil, "2026-11-16 10:00")
	if len(fb.created) != 1 {
		t.Fatalf("shadow não pode mudar a decisão: esperado 1 chamado, veio %d", len(fb.created))
	}
	if got := field(fb.created[0], "urgency", "name"); got != "Média" {
		t.Errorf("urgency = %q, esperado Média (rule)", got)
	}
	for _, p := range fb.patches {
		if a, _ := p["action"].(string); strings.Contains(a, "Instrução:") {
			t.Error("shadow não pode registrar instrução")
		}
	}
}

func TestRunFluxoInexistenteSegueARule(t *testing.T) {
	fb := runFlowAlert(t, "nao-existe", "", "Information", nil, "2026-11-16 10:00")
	if len(fb.created) != 1 || field(fb.created[0], "urgency", "name") != "Média" {
		t.Fatalf("fluxo inexistente deve seguir a rule fixa: %v", fb.created)
	}
}

func TestSeverityLevel(t *testing.T) {
	for in, want := range map[string]int{"Disaster": 5, "alta": 4, "Média": 3, "Warning": 2, "Information": 1, "Not classified": 0, "3": 3, "?": -1} {
		if got := severityLevel(in); got != want {
			t.Errorf("severityLevel(%q) = %d, esperado %d", in, got, want)
		}
	}
}
