package flow

import (
	"strings"
	"testing"

	"godesk/internal/config"
)

// leva 1: condição múltipla, grupos, operadores novos, transformar texto
const logicYAML = `
flows:
  - name: f
    nodes:
      - {id: s, type: start}
      - id: tx
        type: transform
        transforms:
          - {var: iface, from: "{trigger}", op: extract, arg: 'interface (\S+)'}
          - {var: site, from: "{event.tag.Site}", op: default, arg: "SEM-SITE"}
          - {var: up, from: "{var.site}", op: upper}
          - {var: msg, from: "Porta {var.iface} em {var.up}", op: replace, arg: " em ", arg2: " @ "}
      - id: br
        type: branch
        branches:
          - id: noite
            label: Madrugada crítica
            match: all
            conditions:
              - {field: now.time, op: between, value: "22:00..06:00"}
              - match: any
                conditions:
                  - {field: host.tag.Criticidade, op: eq, value: Critico}
                  - {field: host.group, op: starts, value: "core/"}
          - id: idade
            conditions: [{field: event.age, op: between, value: "30..60"}]
          - id: re
            conditions: [{field: host, op: regex, value: '^sw_(core|dist)_\d+$'}]
      - {id: a, type: set, set: [{field: instrucao, value: "noite: {var.msg}"}]}
      - {id: b, type: set, set: [{field: instrucao, value: "idade"}]}
      - {id: c, type: set, set: [{field: instrucao, value: "regex"}]}
      - {id: d, type: set, set: [{field: instrucao, value: "senão"}]}
    links:
      - {from: s, to: tx}
      - {from: tx, to: br}
      - {from: br, port: noite, to: a}
      - {from: br, port: idade, to: b}
      - {from: br, port: re, to: c}
      - {from: br, port: else, to: d}
`

func runLogic(t *testing.T, ev fakeEvent, now string) Result {
	t.Helper()
	f, err := config.ParseFlows([]byte(logicYAML))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fl, _ := f.Find("f")
	r, err := Run(f, fl, ev, at(now))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestCondicaoMultiplaEGrupos(t *testing.T) {
	// 23h + crítico: primeiro ramo (grupo OU dentro do E)
	r := runLogic(t, fakeEvent{"host.tag.Criticidade": {"critico"}, "trigger": {"Link interface Gi0/1 down"}, "event.tag.Site": {"uhcc"}}, "2026-11-16 23:30")
	want(t, r, "instrucao", "noite: Porta Gi0/1 @ UHCC")
	if !strings.Contains(r.Path(), "br[noite]") {
		t.Errorf("caminho: %s", r.Path())
	}
	if r.Vars["iface"] != "Gi0/1" || r.Vars["site"] != "uhcc" {
		t.Errorf("vars: %v", r.Vars)
	}

	// 03h, grupo core/: também noite (virada da meia-noite)
	r = runLogic(t, fakeEvent{"host.group": {"Core/UHCC"}}, "2026-11-16 03:00")
	want(t, r, "instrucao", "noite: Porta  @ SEM-SITE")

	// 10h com idade 45: segundo ramo
	r = runLogic(t, fakeEvent{"host.tag.Criticidade": {"Critico"}, "event.age": {"45"}}, "2026-11-16 10:00")
	want(t, r, "instrucao", "idade")

	// idade 61 (fora) e host casa a regex: terceiro ramo
	r = runLogic(t, fakeEvent{"event.age": {"61"}, "host": {"SW_CORE_12"}}, "2026-11-16 10:00")
	want(t, r, "instrucao", "regex")

	// nada: senão
	r = runLogic(t, fakeEvent{"host": {"sw_core_x"}}, "2026-11-16 10:00")
	want(t, r, "instrucao", "senão")
	if !strings.Contains(r.Path(), "br[else]") {
		t.Errorf("caminho: %s", r.Path())
	}
}

func TestOperadoresNovos(t *testing.T) {
	cases := []struct {
		vals []string
		op   string
		want string
		ok   bool
	}{
		{[]string{"SW_CORE_1"}, "starts", "sw_", true},
		{[]string{"SW_CORE_1"}, "ends", "_1", true},
		{[]string{"SW_CORE_1"}, "ends", "core", false},
		{[]string{"Gi0/1 down"}, "regex", `^gi\d/\d+`, true},
		{[]string{"Gi0/1 down"}, "not_regex", `^gi\d/\d+`, false},
		{[]string{"x"}, "regex", "(", false},     // regex inválida = não bate
		{[]string{"x"}, "not_regex", "(", false}, // nem o negativo
		{[]string{"45"}, "between", "30..60", true},
		{[]string{"30"}, "between", "30..60", true},
		{[]string{"60,5"}, "between", "30..60", false},
		{[]string{"23:10"}, "between", "22:00..06:00", true},
		{[]string{"05:59"}, "between", "22:00..06:00", true},
		{[]string{"12:00"}, "between", "22:00..06:00", false},
		{[]string{"b"}, "between", "a..c", true},
		{[]string{""}, "between", "a..c", false},
		{[]string{"45"}, "between", "60..30", false}, // invertido = fora de (30, 60), como a virada do horário
		{[]string{"5"}, "between", "60..30", true},
		{[]string{"70"}, "between", "60..30", true},
		{[]string{"5"}, "between", "30", false}, // formato inválido
	}
	for _, c := range cases {
		if got := EvalCondition(c.vals, c.op, c.want); got != c.ok {
			t.Errorf("%v %s %q = %v, esperado %v", c.vals, c.op, c.want, got, c.ok)
		}
	}
}

func TestValidacaoLogica(t *testing.T) {
	head := "flows:\n  - name: f\n    nodes:\n      - {id: s, type: start}\n"
	bad := map[string]string{
		"branch sem ramos":     "      - {id: b, type: branch}\n",
		"ramo sem regras":      "      - {id: b, type: branch, branches: [{id: x}]}\n",
		"ramo repetido":        "      - {id: b, type: branch, branches: [{id: x, conditions: [{field: host, op: exists}]}, {id: X, conditions: [{field: host, op: exists}]}]}\n",
		"ramo reservado":       "      - {id: b, type: branch, branches: [{id: else, conditions: [{field: host, op: exists}]}]}\n",
		"regex inválida":       "      - {id: c, type: if, conditions: [{field: host, op: regex, value: '('}]}\n",
		"between sem ..":       "      - {id: c, type: if, conditions: [{field: host, op: between, value: '10'}]}\n",
		"grupo match":          "      - {id: c, type: if, conditions: [{match: talvez, conditions: [{field: host, op: exists}]}]}\n",
		"grupo regra inválida": "      - {id: c, type: if, conditions: [{match: any, conditions: [{field: host, op: parecido}]}]}\n",
		"transform var":        "      - {id: t, type: transform, transforms: [{var: 'a-b', from: x}]}\n",
		"transform op":         "      - {id: t, type: transform, transforms: [{var: a, from: x, op: base64}]}\n",
		"transform extract":    "      - {id: t, type: transform, transforms: [{var: a, from: x, op: extract, arg: '('}]}\n",
		"transform replace":    "      - {id: t, type: transform, transforms: [{var: a, from: x, op: replace}]}\n",
		"transform vazio":      "      - {id: t, type: transform}\n",
		"grupos fundos demais": "      - {id: c, type: if, conditions: [{match: any, conditions: [{match: any, conditions: [{match: any, conditions: [{match: any, conditions: [{field: host, op: exists}]}]}]}]}]}\n",
	}
	for name, nodes := range bad {
		if _, err := config.ParseFlows([]byte(head + nodes + "    links: []\n")); err == nil {
			t.Errorf("%s: esperado erro de validação", name)
		}
	}
	// ligação pela saída do ramo e pelo senão
	ok := head + "      - {id: b, type: branch, branches: [{id: Alta, conditions: [{field: host, op: exists}]}]}\n      - {id: e, type: end}\n" +
		"    links: [{from: s, to: b}, {from: b, port: alta, to: e}, {from: b, port: else, to: e}]\n"
	f, err := config.ParseFlows([]byte(ok))
	if err != nil {
		t.Fatalf("válido recusado: %v", err)
	}
	if f.Flows[0].Links[1].Port != "Alta" {
		t.Errorf("porta do ramo não normalizada: %q", f.Flows[0].Links[1].Port)
	}
	// regex com variável só é conferida na hora (não recusa)
	if _, err := config.ParseFlows([]byte(head + "      - {id: c, type: if, conditions: [{field: host, op: regex, value: '{var.x}'}]}\n    links: []\n")); err != nil {
		t.Errorf("regex com variável recusada: %v", err)
	}
}
