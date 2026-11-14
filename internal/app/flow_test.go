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
	"godesk/internal/flowlog"
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

  - name: escala
    nodes:
      - {id: s, type: start}
      - {id: quieto, type: set, set: [{field: repeat_update, value: "false"}]}
      - {id: idade, type: if, conditions: [{field: event.age, op: gte, value: "30"}, {field: event.acked_by_user, op: is_false}]}
      - id: escalar
        type: update
        set: [{field: urgency, value: Crítica}, {field: oper_group, value: N2}]
        note: "Sem ack há {event.age} min (reenvio {event.resends})"
        teams: true
    links:
      - {from: s, to: quieto}
      - {from: quieto, to: idade}
      - {from: idade, port: "true", to: escalar}

  - name: tabela
    nodes:
      - {id: s, type: start}
      - id: t
        type: table
        field: host.group
        columns: [oper_group, contract, teams_title]
        rows:
          - {match: "ESCRITORIO/*", values: [G_ESC, C_ESC]}
          - {match: "POWERPLANT/*", values: [G_USINA, C_USINA, "Usina: {host}"]}
      - {id: http, type: http, method: POST, url: "__HTTP__/hook?h={host|url}", headers: ["X-Teste: {rule}"], body: '{"chamado":"{ticket.number}","trigger":"{trigger|json}"}'}
    links:
      - {from: s, to: t}
      - {from: t, port: found, to: http}
`

func flowTestPolicies(flow, mode, webhook string) string {
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
    topdesk: {send_teams: true, teams_webhook: "` + webhook + `"}
`
}

type httpHit struct {
	path    string
	header  http.Header
	body    string
	query   string
	created time.Time
}

// fakeBackend responde como TopDesk (/tas/api/...), Zabbix
// (/api_jsonrpc.php), Teams (/teams*) e destino do bloco http (/hook).
type fakeBackend struct {
	mu       sync.Mutex
	created  []map[string]any
	patches  []map[string]any
	zbxCalls []string
	acks     []string
	teams    []httpHit
	hooks    []httpHit

	hostTags     []map[string]string
	ticketExists bool
	eventClock   time.Time
	eventAcks    []map[string]string
}

func (f *fakeBackend) handler(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	body, _ := io.ReadAll(r.Body)

	switch {
	case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/tas/api/incidents"):
		if f.ticketExists {
			_, _ = w.Write([]byte(`[{"number":"I2611-001","processingStatus":{"name":"Em andamento"}}]`))
			return
		}
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
	case strings.HasPrefix(r.URL.Path, "/teams"):
		f.teams = append(f.teams, httpHit{path: r.URL.Path, body: string(body)})
		w.WriteHeader(http.StatusAccepted)
	case r.URL.Path == "/hook":
		f.hooks = append(f.hooks, httpHit{path: r.URL.Path, header: r.Header.Clone(), body: string(body), query: r.URL.RawQuery})
		_, _ = w.Write([]byte(`ok`))
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
				"tags": tags, "inheritedTags": []any{map[string]any{"tag": "Herdada", "value": "sim"}},
				"hostgroups": []any{map[string]any{"name": "POWERPLANT/GERACAO/UHCC"}},
			}}
		case "event.get":
			acks := []any{}
			acked := "0"
			for _, a := range f.eventAcks {
				acks = append(acks, map[string]any{"userid": a["userid"], "action": a["action"], "message": a["message"]})
				acked = "1"
			}
			clock := f.eventClock
			if clock.IsZero() {
				clock = time.Now()
			}
			result = []any{map[string]any{"eventid": "1", "clock": itoa(clock.Unix()), "acknowledged": acked, "tags": []any{}, "acknowledges": acks}}
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

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

type alertOpts struct {
	flow, mode, severity, now string
	hostTags                  []map[string]string
	fb                        *fakeBackend
	dir                       string
}

func runFlowAlert(t *testing.T, o alertOpts) *fakeBackend {
	t.Helper()
	if o.dir == "" {
		o.dir = t.TempDir()
	}
	fb := o.fb
	if fb == nil {
		fb = &fakeBackend{}
	}
	if o.hostTags != nil {
		fb.hostTags = o.hostTags
	}
	srv := httptest.NewServer(http.HandlerFunc(fb.handler))
	defer srv.Close()

	cfgPath := filepath.Join(o.dir, "godesk-config.yaml")
	flowsPath := filepath.Join(o.dir, "godesk-flows.yaml")
	webhook := srv.URL + "/teams1, " + srv.URL + "/teams2"
	if err := os.WriteFile(cfgPath, []byte(flowTestPolicies(o.flow, o.mode, webhook)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(flowsPath, []byte(strings.ReplaceAll(flowTestFlows, "__HTTP__", srv.URL)), 0o644); err != nil {
		t.Fatal(err)
	}

	when, err := time.ParseInLocation("2006-01-02 15:04", o.now, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	prev := flowNow
	flowNow = func() time.Time { return when }
	defer func() { flowNow = prev }()

	sev := o.severity
	if sev == "" {
		sev = "High"
	}
	raw, _ := json.Marshal(map[string]string{
		"status": "PROBLEM", "host": "SW_CORE_UHCC_1", "host_id": "10500",
		"trigger": `Unavailable "by" ICMP ping`, "severity": sev,
		"event_id": "1", "trigger_id": "2", "event_value": "1", "rule_name": "infra",
		"date": "2026.11.16", "hour": "10:00:00",
	})

	err = Run(config.RuntimeConfig{
		Domain: srv.URL, User: "u", Pass: "p",
		TicketName: "[UHCC]: Unavailable by ICMP ping - SW_CORE_UHCC_1",
		RawData:    string(raw),
		ZabbixURL:  srv.URL, ZabbixKey: "k",
		ConfigFile: cfgPath, FlowsFile: flowsPath,
		FlowsHistoryFile: filepath.Join(o.dir, "history.jsonl"),
		FlowsStateFile:   filepath.Join(o.dir, "state.json"),
		MetricsFile:      filepath.Join(o.dir, "metrics.json"),
		TimeoutSec:       5,
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

func patchWith(fb *fakeBackend, needle string) []map[string]any {
	var out []map[string]any
	for _, p := range fb.patches {
		if a, _ := p["action"].(string); strings.Contains(a, needle) {
			out = append(out, p)
		}
	}
	return out
}

func TestRunFluxoHostCriticoDefineCamposEInstrucao(t *testing.T) {
	fb := runFlowAlert(t, alertOpts{flow: "infra", now: "2026-11-16 10:00",
		hostTags: []map[string]string{{"tag": "Criticidade", "value": "Critico"}, {"tag": "Grupo", "value": "GRUPO_CRIT"}}})

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
	ins := patchWith(fb, "Instrução:")
	if len(ins) != 1 || !strings.Contains(ins[0]["action"].(string), "Host crítico (Switch Core UHCC 1): LIGAR para o cliente") {
		t.Fatalf("instrução não registrada: %v", fb.patches)
	}
	if inv, _ := ins[0]["actionInvisibleForCaller"].(bool); !inv {
		t.Error("instrução precisa ser invisível pro solicitante")
	}
	// Teams nos dois webhooks, com a instrução em destaque
	if len(fb.teams) != 2 || !strings.Contains(fb.teams[0].body, "Instrução: Host crítico") {
		t.Errorf("Teams: %d envios %v", len(fb.teams), fb.teams)
	}
}

func TestRunFluxoInformativoNaoAbre(t *testing.T) {
	dir := t.TempDir()
	fb := runFlowAlert(t, alertOpts{flow: "infra", mode: "on", severity: "Information", now: "2026-11-16 10:00", dir: dir})
	if len(fb.created) != 0 {
		t.Fatalf("não devia abrir chamado, abriu %d", len(fb.created))
	}
	if len(fb.acks) != 1 || !strings.Contains(fb.acks[0], `Unavailable "by" ICMP ping informativo`) {
		t.Errorf("comentário no evento = %v", fb.acks)
	}
	h, _ := flowlog.List(filepath.Join(dir, "history.jsonl"), flowlog.Filter{})
	if len(h) != 1 || h[0].Action != "skip" || !strings.Contains(strings.Join(h[0].Notes, ";"), "não aberto") {
		t.Errorf("histórico: %+v", h)
	}
}

func TestRunFluxoForaDoHorario(t *testing.T) {
	fb := runFlowAlert(t, alertOpts{flow: "infra", now: "2026-11-14 22:00"})
	if len(fb.created) != 1 || field(fb.created[0], "urgency", "name") != "Alta" {
		t.Fatalf("esperado chamado com urgência Alta: %v", fb.created)
	}
	for _, m := range fb.zbxCalls {
		if m == "host.get" {
			t.Errorf("host.get não devia ser chamado (fluxo não chegou no switch de tag)")
		}
	}
}

func TestRunFluxoShadowNaoAlteraMasRegistra(t *testing.T) {
	dir := t.TempDir()
	fb := runFlowAlert(t, alertOpts{flow: "infra", mode: "shadow", severity: "Information", now: "2026-11-16 10:00", dir: dir})
	if len(fb.created) != 1 || field(fb.created[0], "urgency", "name") != "Média" {
		t.Fatalf("shadow não pode mudar a decisão: %v", fb.created)
	}
	if len(patchWith(fb, "Instrução:")) != 0 {
		t.Error("shadow não pode registrar instrução")
	}
	h, _ := flowlog.List(filepath.Join(dir, "history.jsonl"), flowlog.Filter{})
	if len(h) != 1 || h[0].Mode != "shadow" || h[0].Action != "skip" || h[0].Ticket != "I2611-001" {
		t.Errorf("histórico do shadow: %+v", h)
	}
}

func TestRunFluxoInexistenteSegueARuleERegistraErro(t *testing.T) {
	dir := t.TempDir()
	fb := runFlowAlert(t, alertOpts{flow: "nao-existe", severity: "Information", now: "2026-11-16 10:00", dir: dir})
	if len(fb.created) != 1 || field(fb.created[0], "urgency", "name") != "Média" {
		t.Fatalf("fluxo inexistente deve seguir a rule fixa: %v", fb.created)
	}
	h, _ := flowlog.List(filepath.Join(dir, "history.jsonl"), flowlog.Filter{})
	if len(h) != 1 || !strings.Contains(h[0].Error, "não existe") {
		t.Errorf("erro não foi pro histórico: %+v", h)
	}
}

func TestRunEscalonamentoUmaVezPorEvento(t *testing.T) {
	dir := t.TempDir()
	start := time.Date(2026, 11, 16, 10, 0, 0, 0, time.Local)
	fb := &fakeBackend{ticketExists: true, eventClock: start,
		eventAcks: []map[string]string{{"userid": "1", "action": "6", "message": "Chamado criado: I2611-001"}}}

	// 20 min: ainda não escala; repeat_update=false → sem "Recebemos novamente"
	runFlowAlert(t, alertOpts{flow: "escala", now: "2026-11-16 10:20", fb: fb, dir: dir})
	if len(fb.patches) != 0 {
		t.Fatalf("aos 20 min não devia mexer no chamado: %v", fb.patches)
	}

	// 35 min: escala (urgência, grupo, nota, Teams)
	runFlowAlert(t, alertOpts{flow: "escala", now: "2026-11-16 10:35", fb: fb, dir: dir})
	esc := patchWith(fb, "Escalonamento:")
	if len(esc) != 1 {
		t.Fatalf("esperado 1 escalonamento, patches=%v", fb.patches)
	}
	if field(esc[0], "urgency", "name") != "Crítica" || field(esc[0], "operatorGroup", "id") != "N2" {
		t.Errorf("campos do escalonamento: %v", esc[0])
	}
	if a := esc[0]["action"].(string); !strings.Contains(a, "Sem ack há 35 min (reenvio 1)") {
		t.Errorf("nota = %q", a)
	}
	if len(fb.teams) != 2 || !strings.Contains(fb.teams[0].body, "Escalonamento:") {
		t.Errorf("Teams do escalonamento: %v", fb.teams)
	}

	// 45 min: já escalou neste evento → não repete
	runFlowAlert(t, alertOpts{flow: "escala", now: "2026-11-16 10:45", fb: fb, dir: dir})
	if len(patchWith(fb, "Escalonamento:")) != 1 || len(fb.teams) != 2 {
		t.Errorf("escalou de novo: patches=%v teams=%d", fb.patches, len(fb.teams))
	}
	h, _ := flowlog.List(filepath.Join(dir, "history.jsonl"), flowlog.Filter{})
	if len(h) != 3 || !strings.Contains(strings.Join(h[0].Notes, ";"), "já aplicado") {
		t.Errorf("histórico do escalonamento: %+v", h)
	}
}

func TestRunEscalonamentoNaoEscalaComAckDoAtendimento(t *testing.T) {
	fb := &fakeBackend{ticketExists: true, eventClock: time.Date(2026, 11, 16, 10, 0, 0, 0, time.Local),
		eventAcks: []map[string]string{
			{"userid": "1", "action": "6", "message": "Chamado criado: I2611-001"},
			{"userid": "7", "action": "2", "message": "assumido"},
		}}
	runFlowAlert(t, alertOpts{flow: "escala", now: "2026-11-16 11:00", fb: fb})
	if len(patchWith(fb, "Escalonamento:")) != 0 {
		t.Errorf("com ack do atendimento não devia escalar: %v", fb.patches)
	}
}

func TestRunTabelaEHTTP(t *testing.T) {
	dir := t.TempDir()
	fb := runFlowAlert(t, alertOpts{flow: "tabela", now: "2026-11-16 10:00", dir: dir})
	if len(fb.created) != 1 {
		t.Fatalf("esperado 1 chamado: %v", fb.created)
	}
	c := fb.created[0]
	if field(c, "operatorGroup", "id") != "G_USINA" {
		t.Errorf("tabela não definiu o grupo: %v", c["operatorGroup"])
	}
	if len(fb.teams) == 0 || !strings.Contains(fb.teams[0].body, "Usina: SW_CORE_UHCC_1") {
		t.Errorf("teams_title da tabela: %v", fb.teams)
	}
	if len(fb.hooks) != 1 {
		t.Fatalf("bloco http não chamado: %v", fb.hooks)
	}
	hk := fb.hooks[0]
	var got map[string]string
	if err := json.Unmarshal([]byte(hk.body), &got); err != nil {
		t.Fatalf("corpo não é JSON válido (|json): %q", hk.body)
	}
	if got["chamado"] != "I2611-001" || got["trigger"] != `Unavailable "by" ICMP ping` {
		t.Errorf("corpo = %v", got)
	}
	if hk.query != "h=SW_CORE_UHCC_1" || hk.header.Get("X-Teste") != "infra" || hk.header.Get("Content-Type") != "application/json" {
		t.Errorf("query=%q header=%v", hk.query, hk.header)
	}
	h, _ := flowlog.List(filepath.Join(dir, "history.jsonl"), flowlog.Filter{})
	if len(h) != 1 || !strings.Contains(strings.Join(h[0].Notes, ";"), "HTTP 200") {
		t.Errorf("resultado do http no histórico: %+v", h)
	}
}

func TestSeverityLevel(t *testing.T) {
	for in, want := range map[string]int{"Disaster": 5, "alta": 4, "Média": 3, "Warning": 2, "Information": 1, "Not classified": 0, "3": 3, "?": -1} {
		if got := severityLevel(in); got != want {
			t.Errorf("severityLevel(%q) = %d, esperado %d", in, got, want)
		}
	}
}
