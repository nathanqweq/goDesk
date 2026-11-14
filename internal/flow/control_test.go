package flow

import (
	"strings"
	"testing"
	"time"

	"godesk/internal/config"
)

// leva 4: subfluxo, limitar repetição, e-mail, nota no desenho
const controlYAML = `
flows:
  - name: principal
    nodes:
      - {id: s, type: start}
      - {id: nota, type: comment, text: "isto é só documentação"}
      - {id: lim, type: limit, max: 2, window_hours: 4}
      - {id: sub, type: subflow, flow: padrao}
      - {id: depois, type: set, set: [{field: equipe, value: "N1 ({set.urgency})"}]}
      - {id: mail, type: email, to: "noc@x.com, {host.tag.Dono}", subject: "[{severity}] {host}", body: "Olá,\nalerta {trigger}"}
      - {id: nao, type: end, action: skip, comment: "repetido: {host}"}
    links:
      - {from: s, to: lim}
      - {from: lim, port: ok, to: sub}
      - {from: lim, port: over, to: nao}
      - {from: sub, to: depois}
      - {from: depois, to: mail}

  - name: padrao
    nodes:
      - {id: s, type: start}
      - {id: c, type: if, conditions: [{field: severity.level, op: lte, value: "1"}]}
      - {id: info, type: end, action: skip, comment: "informativo"}
      - {id: alta, type: set, set: [{field: urgency, value: Alta}]}
      - {id: fim, type: end}
    links:
      - {from: s, to: c}
      - {from: c, port: "true", to: info}
      - {from: c, port: "false", to: alta}
      - {from: alta, to: fim}
`

type limitedEvent struct {
	fakeEvent
	allowed bool
	count   int
	keys    []string
}

func (l *limitedEvent) Allow(key string, max int, window time.Duration) (bool, int) {
	l.keys = append(l.keys, key)
	return l.allowed, l.count
}

func runControl(t *testing.T, res Resolver) Result {
	t.Helper()
	f, err := config.ParseFlows([]byte(controlYAML))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fl, _ := f.Find("principal")
	r, err := Run(f, fl, res, at("2026-11-16 10:00"))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestSubfluxoELimite(t *testing.T) {
	ev := &limitedEvent{fakeEvent: fakeEvent{"host": {"SW1"}, "trigger": {"Ping"}, "severity": {"High"}, "severity.level": {"4"}, "host.tag.Dono": {"dono@x.com"}}, allowed: true, count: 1}
	r := runControl(t, ev)
	// subfluxo define a urgência, volta e o principal continua
	want(t, r, "urgency", "Alta")
	want(t, r, "equipe", "N1 (Alta)")
	if r.Action != config.FlowActionOpen {
		t.Errorf("action = %s", r.Action)
	}
	if !strings.Contains(r.Path(), "lim[ok] → sub → depois → mail") {
		t.Errorf("caminho: %s", r.Path())
	}
	var subStep Step
	for _, s := range r.Trace {
		if s.Node == "sub" {
			subStep = s
		}
	}
	if !strings.Contains(subStep.Detail, "fluxo padrao: s → c[false] → alta → fim") {
		t.Errorf("detalhe do subfluxo: %q", subStep.Detail)
	}
	if len(ev.keys) != 1 || ev.keys[0] != "SW1|Ping" {
		t.Errorf("chave do limite: %v", ev.keys)
	}
	if len(r.Emails) != 1 || r.Emails[0].To != "noc@x.com, dono@x.com" || r.Emails[0].Subject != "[High] SW1" || r.Emails[0].Body != "Olá,\nalerta Ping" {
		t.Errorf("e-mail: %+v", r.Emails)
	}

	// subfluxo "não abrir" encerra tudo
	ev = &limitedEvent{fakeEvent: fakeEvent{"host": {"SW1"}, "severity.level": {"1"}}, allowed: true}
	r = runControl(t, ev)
	if r.Action != config.FlowActionSkip || r.Comment != "informativo" || len(r.Emails) != 0 {
		t.Errorf("skip do subfluxo não encerrou: %s %q emails=%d caminho=%s", r.Action, r.Comment, len(r.Emails), r.Path())
	}

	// passou do limite
	ev = &limitedEvent{fakeEvent: fakeEvent{"host": {"SW1"}}, allowed: false, count: 2}
	r = runControl(t, ev)
	if r.Action != config.FlowActionSkip || r.Comment != "repetido: SW1" {
		t.Errorf("acima do limite: %s %q", r.Action, r.Comment)
	}

	// sem Limiter (simulador): sempre dentro
	r = runControl(t, fakeEvent{"host": {"SW1"}, "severity.level": {"4"}})
	if !strings.Contains(r.Path(), "lim[ok]") {
		t.Errorf("sem limiter devia ser dentro: %s", r.Path())
	}
}

func TestValidacaoControle(t *testing.T) {
	cases := map[string]string{
		"subfluxo inexistente": "flows:\n  - name: a\n    nodes: [{id: s, type: start}, {id: x, type: subflow, flow: nada}]\n    links: [{from: s, to: x}]\n",
		"subfluxo vazio":       "flows:\n  - name: a\n    nodes: [{id: s, type: start}, {id: x, type: subflow}]\n    links: []\n",
		"ciclo de subfluxos": "flows:\n  - name: a\n    nodes: [{id: s, type: start}, {id: x, type: subflow, flow: b}]\n    links: [{from: s, to: x}]\n" +
			"  - name: b\n    nodes: [{id: s, type: start}, {id: x, type: subflow, flow: A}]\n    links: [{from: s, to: x}]\n",
		"subfluxo de si mesmo": "flows:\n  - name: a\n    nodes: [{id: s, type: start}, {id: x, type: subflow, flow: a}]\n    links: []\n",
		"nota ligada":          "flows:\n  - name: a\n    nodes: [{id: s, type: start}, {id: n, type: comment, text: x}]\n    links: [{from: s, to: n}]\n",
		"limite zero":          "flows:\n  - name: a\n    nodes: [{id: s, type: start}, {id: l, type: limit, max: 0, window_hours: 1}]\n    links: []\n",
		"janela grande":        "flows:\n  - name: a\n    nodes: [{id: s, type: start}, {id: l, type: limit, max: 1, window_hours: 500}]\n    links: []\n",
		"e-mail sem para":      "flows:\n  - name: a\n    nodes: [{id: s, type: start}, {id: m, type: email, subject: x}]\n    links: []\n",
		"e-mail sem assunto":   "flows:\n  - name: a\n    nodes: [{id: s, type: start}, {id: m, type: email, to: a@b}]\n    links: []\n",
	}
	for name, y := range cases {
		if _, err := config.ParseFlows([]byte(y)); err == nil {
			t.Errorf("%s: esperado erro de validação", name)
		}
	}
}
