package flow

import (
	"strings"
	"testing"
	"time"

	"godesk/internal/config"
)

// Exemplo real: fora do horário comercial liga sempre; no horário, host
// crítico liga, alto só Teams, informativo não abre chamado.
const infraYAML = `
calendars:
  comercial:
    days: [seg, ter, qua, qui, sex]
    start: "08:00"
    end: "18:00"
    holidays: ["2026-12-25"]
flows:
  - name: infra
    nodes:
      - {id: inicio, type: start}
      - id: horario
        type: if
        label: Fora do horário?
        conditions: [{field: calendar.comercial, op: is_false}]
      - id: fora
        type: set
        set:
          - {field: instrucao, value: "Fora do horário comercial: LIGAR para o cliente ({host})"}
          - {field: urgency, value: Alta}
      - id: crit
        type: switch
        field: host.tag.Criticidade
        cases: [{value: Critico}, {value: "Alto,Alta"}]
      - id: ligar
        type: set
        set: [{field: instrucao, value: "Host crítico: LIGAR para o cliente"}, {field: urgency, value: Alta}]
      - id: teams
        type: set
        set: [{field: instrucao, value: "Avisar no Teams"}, {field: send_teams, value: "true"}]
      - id: info
        type: if
        conditions: [{field: severity, op: in, value: "Information,Not classified"}]
      - id: nao
        type: end
        action: skip
        comment: "goDesk: {trigger} é informativo, sem chamado"
    links:
      - {from: inicio, to: horario}
      - {from: horario, port: "true", to: fora}
      - {from: horario, port: "false", to: crit}
      - {from: crit, port: Critico, to: ligar}
      - {from: crit, port: "Alto,Alta", to: teams}
      - {from: crit, port: default, to: info}
      - {from: info, port: "true", to: nao}
`

type fakeEvent map[string][]string

func (f fakeEvent) Values(field string) []string { return f[field] }

func run(t *testing.T, ev fakeEvent, now time.Time) Result {
	t.Helper()
	f, err := config.ParseFlows([]byte(infraYAML))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fl, ok := f.Find("INFRA")
	if !ok {
		t.Fatal("fluxo infra não encontrado")
	}
	r, err := Run(f, fl, ev, now)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return r
}

func at(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local)
	if err != nil {
		panic(err)
	}
	return t
}

func want(t *testing.T, r Result, field, value string) {
	t.Helper()
	got, ok := r.Value(field)
	if !ok || got != value {
		t.Errorf("%s = %q (definido=%v), esperado %q — caminho: %s", field, got, ok, value, r.Path())
	}
}

func TestForaDoHorarioLigaSempre(t *testing.T) {
	// sábado
	r := run(t, fakeEvent{"host": {"SW_CORE_UHCC_1"}, "host.tag.Criticidade": {"Baixo"}}, at("2026-11-14 10:00"))
	want(t, r, "instrucao", "Fora do horário comercial: LIGAR para o cliente (SW_CORE_UHCC_1)")
	want(t, r, "urgency", "Alta")
	if r.Action != config.FlowActionOpen {
		t.Errorf("action = %s", r.Action)
	}
	if !strings.Contains(r.Path(), "Fora do horário?[true]") {
		t.Errorf("caminho: %s", r.Path())
	}
}

func TestFeriadoEFimDoExpedienteContamComoForaDoHorario(t *testing.T) {
	for _, now := range []string{"2026-12-25 10:00", "2026-11-16 18:00", "2026-11-16 07:59"} {
		r := run(t, fakeEvent{"host": {"X"}}, at(now))
		if _, ok := r.Value("urgency"); !ok {
			t.Errorf("%s: deveria cair em fora do horário — caminho: %s", now, r.Path())
		}
	}
}

func TestNoHorarioHostCriticoLiga(t *testing.T) {
	r := run(t, fakeEvent{"host.tag.Criticidade": {"critico"}}, at("2026-11-16 10:00"))
	want(t, r, "instrucao", "Host crítico: LIGAR para o cliente")
}

func TestCasoComListaEMaiusculas(t *testing.T) {
	r := run(t, fakeEvent{"host.tag.Criticidade": {"ALTA"}}, at("2026-11-16 10:00"))
	want(t, r, "instrucao", "Avisar no Teams")
	want(t, r, "send_teams", "true")
	if !strings.Contains(r.Path(), "crit[Alto,Alta]") {
		t.Errorf("caminho: %s", r.Path())
	}
}

func TestInformativoNaoAbreChamado(t *testing.T) {
	r := run(t, fakeEvent{"severity": {"Information"}, "trigger": {"Disco 80%"}}, at("2026-11-16 10:00"))
	if r.Action != config.FlowActionSkip {
		t.Fatalf("action = %s, esperado skip — caminho: %s", r.Action, r.Path())
	}
	if r.Comment != "goDesk: Disco 80% é informativo, sem chamado" {
		t.Errorf("comment = %q", r.Comment)
	}
}

func TestPortaSemLigacaoSegueARule(t *testing.T) {
	// sem tag e severidade alta: crit → default → info → false (sem ligação)
	r := run(t, fakeEvent{"severity": {"High"}}, at("2026-11-16 10:00"))
	if r.Action != config.FlowActionOpen || len(r.Set) != 0 {
		t.Errorf("esperado abrir sem campos, veio %s %+v — caminho: %s", r.Action, r.Set, r.Path())
	}
}

func TestEvalCondition(t *testing.T) {
	groups := []string{"POWERPLANT/GERACAO/UHCC", "Switch/Core"}
	cases := []struct {
		vals []string
		op   string
		want string
		ok   bool
	}{
		{groups, "matches", "powerplant/*", true},
		{groups, "not_matches", "ESCRITORIO/*", true},
		{groups, "neq", "Switch/Core", false}, // negativo: nenhum pode bater
		{groups, "in", "Router, switch/*", true},
		{groups, "contains", "geracao", true},
		{nil, "not_exists", "", true},
		{[]string{""}, "exists", "", false},
		{[]string{"4"}, "gte", "4", true},
		{[]string{"3,5"}, "gt", "3", true},
		{[]string{"abc"}, "gt", "1", false},
		{[]string{"sim"}, "is_true", "", true},
		{nil, "is_false", "", true},
	}
	for _, c := range cases {
		if got := EvalCondition(c.vals, c.op, c.want); got != c.ok {
			t.Errorf("%v %s %q = %v, esperado %v", c.vals, c.op, c.want, got, c.ok)
		}
	}
}

func TestWildcard(t *testing.T) {
	for _, c := range []struct {
		p, s string
		ok   bool
	}{
		{"*Unavailable by ICMP ping*", "SW Unavailable by ICMP ping", true},
		{"a*b*c", "aXXbYYc", true},
		{"a*b*c", "aXXcYYb", false},
		{"abc", "ABC", true},
		{"*", "", true},
	} {
		if got := Wildcard(c.p, c.s); got != c.ok {
			t.Errorf("Wildcard(%q, %q) = %v", c.p, c.s, got)
		}
	}
}

func TestSetLidoMaisAdiante(t *testing.T) {
	y := `
flows:
  - name: f
    nodes:
      - {id: s, type: start}
      - {id: a, type: set, set: [{field: urgency, value: Alta}]}
      - {id: b, type: if, conditions: [{field: set.urgency, op: eq, value: alta}]}
      - {id: c, type: set, set: [{field: instrucao, value: "urgência {set.urgency} em {now.weekday}"}]}
    links:
      - {from: s, to: a}
      - {from: a, to: b}
      - {from: b, port: "true", to: c}
`
	f, err := config.ParseFlows([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	fl, _ := f.Find("f")
	r, err := Run(f, fl, nil, at("2026-11-16 10:00"))
	if err != nil {
		t.Fatal(err)
	}
	want(t, r, "instrucao", "urgência Alta em seg")
}

func TestValidacao(t *testing.T) {
	base := func(nodes, links string) string {
		return "calendars:\n  comercial: {days: [seg], start: \"08:00\", end: \"18:00\"}\nflows:\n  - name: f\n    nodes:\n" + nodes + "    links:\n" + links
	}
	start := "      - {id: s, type: start}\n"
	cases := map[string]string{
		"ciclo": base(start+"      - {id: a, type: if, conditions: [{field: host, op: eq, value: x}]}\n      - {id: b, type: set, set: [{field: urgency, value: Alta}]}\n",
			"      - {from: s, to: a}\n      - {from: a, port: \"true\", to: b}\n      - {from: b, to: a}\n"),
		"porta inexistente": base(start+"      - {id: a, type: set, set: [{field: urgency, value: Alta}]}\n",
			"      - {from: s, port: \"true\", to: a}\n"),
		"dois inícios":        base(start+"      - {id: s2, type: start}\n", "      []\n"),
		"campo do set":        base(start+"      - {id: a, type: set, set: [{field: senha, value: x}]}\n", "      []\n"),
		"booleano":            base(start+"      - {id: a, type: set, set: [{field: send_teams, value: talvez}]}\n", "      []\n"),
		"calendário":          base(start+"      - {id: a, type: if, conditions: [{field: calendar.noturno, op: is_true}]}\n", "      []\n"),
		"caso repetido":       base(start+"      - {id: a, type: switch, field: host, cases: [{value: x}, {value: X}]}\n", "      []\n"),
		"liga no início":      base(start+"      - {id: a, type: set, set: [{field: urgency, value: Alta}]}\n", "      - {from: a, to: s}\n"),
		"saída duplicada":     base(start+"      - {id: a, type: end}\n      - {id: b, type: end}\n", "      - {from: s, to: a}\n      - {from: s, to: b}\n"),
		"operador":            base(start+"      - {id: a, type: if, conditions: [{field: host, op: parecido}]}\n", "      []\n"),
		"tipo":                base(start+"      - {id: a, type: loop}\n", "      []\n"),
		"template calendário": base(start+"      - {id: a, type: set, set: [{field: instrucao, value: \"{calendar.x}\"}]}\n", "      []\n"),
	}
	for name, y := range cases {
		if _, err := config.ParseFlows([]byte(y)); err == nil {
			t.Errorf("%s: esperado erro de validação", name)
		}
	}

	// porta do switch sem diferenciar maiúsculas é normalizada pro caso
	ok := base(start+"      - {id: a, type: switch, field: host, cases: [{value: Critico}]}\n      - {id: b, type: end}\n",
		"      - {from: s, to: a}\n      - {from: a, port: critico, to: b}\n")
	f, err := config.ParseFlows([]byte(ok))
	if err != nil {
		t.Fatalf("válido recusado: %v", err)
	}
	if f.Flows[0].Links[1].Port != "Critico" {
		t.Errorf("porta não normalizada: %q", f.Flows[0].Links[1].Port)
	}
}
