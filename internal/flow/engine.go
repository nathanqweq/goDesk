// Package flow executa os fluxos de decisão do godesk-flows.yaml (ver
// internal/config/flows.go). O motor é puro: recebe um Resolver que
// devolve os valores dos campos do evento (host, severidade, tags...) e
// devolve o que o fluxo decidiu — campos definidos, ação (abrir / não
// abrir) e o caminho percorrido, pra log e pro editor.
package flow

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"godesk/internal/config"
)

// Resolver devolve os valores de um campo do evento. Campo multivalorado
// (host.group, tag repetida) devolve vários; desconhecido devolve nil.
type Resolver interface {
	Values(field string) []string
}

// ResolverFunc adapta uma função a Resolver.
type ResolverFunc func(field string) []string

func (f ResolverFunc) Values(field string) []string { return f(field) }

// Step é um bloco percorrido.
type Step struct {
	Node   string `json:"node"`
	Type   string `json:"type"`
	Label  string `json:"label,omitempty"`
	Port   string `json:"port,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// Assignment é um campo definido por um bloco set, já com o template
// resolvido.
type Assignment struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

type Result struct {
	Flow    string       `json:"flow"`
	Action  string       `json:"action"`  // open | skip
	Comment string       `json:"comment"` // comentário no evento (skip)
	Set     []Assignment `json:"set"`
	Trace   []Step       `json:"trace"`
}

// Value devolve o último valor definido pro campo.
func (r Result) Value(field string) (string, bool) {
	for i := len(r.Set) - 1; i >= 0; i-- {
		if r.Set[i].Field == field {
			return r.Set[i].Value, true
		}
	}
	return "", false
}

// Path resume o caminho: "inicio → crit[Critico] → ligar → fim(open)".
func (r Result) Path() string {
	parts := make([]string, 0, len(r.Trace))
	for _, s := range r.Trace {
		name := s.Node
		if s.Label != "" {
			name = s.Label
		}
		if s.Port != "" && s.Port != config.PortNext {
			name += "[" + s.Port + "]"
		}
		parts = append(parts, name)
	}
	return strings.Join(parts, " → ")
}

const maxSteps = 200

var templateRe = regexp.MustCompile(`\{[^{}]+\}`)

// Run executa o fluxo a partir do bloco de início. Porta sem ligação
// termina com "open" (segue a rule com o que já foi definido).
func Run(f config.FlowsFile, fl config.Flow, res Resolver, now time.Time) (Result, error) {
	r := Result{Flow: fl.Name, Action: config.FlowActionOpen}
	ev := evaluator{file: f, res: res, now: now, result: &r}

	cur, ok := fl.Start()
	if !ok {
		return r, fmt.Errorf("fluxo %q sem bloco de início", fl.Name)
	}

	for i := 0; i < maxSteps; i++ {
		step := Step{Node: cur.ID, Type: cur.Type, Label: cur.Label}
		port := ""

		switch cur.Type {
		case config.NodeStart:
			port = config.PortNext

		case config.NodeIf:
			ok, detail := ev.evalIf(cur)
			step.Detail = detail
			port = config.PortFalse
			if ok {
				port = config.PortTrue
			}

		case config.NodeSwitch:
			vals := ev.values(cur.Field)
			port = config.PortDefault
			for _, c := range cur.Cases {
				if matchCase(vals, c.Value) {
					port = c.Value
					break
				}
			}
			step.Detail = fmt.Sprintf("%s = %s", cur.Field, showValues(vals))

		case config.NodeSet:
			var done []string
			for _, a := range cur.Set {
				v := ev.render(a.Value)
				r.Set = append(r.Set, Assignment{Field: a.Field, Value: v})
				done = append(done, a.Field+"="+v)
			}
			step.Detail = strings.Join(done, "; ")
			port = config.PortNext

		case config.NodeEnd:
			r.Action = cur.Action
			if r.Action == "" {
				r.Action = config.FlowActionOpen
			}
			r.Comment = ev.render(cur.Comment)
			step.Detail = r.Action
			r.Trace = append(r.Trace, step)
			return r, nil

		default:
			return r, fmt.Errorf("bloco %q: tipo %q desconhecido", cur.ID, cur.Type)
		}

		step.Port = port
		r.Trace = append(r.Trace, step)

		nextID := fl.Next(cur.ID, port)
		if nextID == "" {
			// fim implícito: segue a rule
			return r, nil
		}
		next, ok := fl.Node(nextID)
		if !ok {
			return r, fmt.Errorf("bloco %q liga em %q, que não existe", cur.ID, nextID)
		}
		cur = next
	}

	return r, fmt.Errorf("fluxo %q passou de %d passos (ciclo?)", fl.Name, maxSteps)
}

type evaluator struct {
	file   config.FlowsFile
	res    Resolver
	now    time.Time
	result *Result
}

var weekdayPT = []string{"dom", "seg", "ter", "qua", "qui", "sex", "sab"}

// values resolve um campo: os do próprio motor (calendar.*, now.*, set.*)
// e o resto pelo Resolver.
func (e evaluator) values(field string) []string {
	field = strings.TrimSpace(field)
	lower := strings.ToLower(field)

	switch {
	case strings.HasPrefix(lower, "calendar."):
		cal, ok := e.file.Calendar(field[len("calendar."):])
		if !ok {
			return nil
		}
		return []string{strconv.FormatBool(InCalendar(cal, e.now))}
	case lower == "now.weekday":
		return []string{weekdayPT[int(e.now.Weekday())]}
	case lower == "now.hour":
		return []string{e.now.Format("15")}
	case lower == "now.time":
		return []string{e.now.Format("15:04")}
	case lower == "now.date":
		return []string{e.now.Format("2006-01-02")}
	case strings.HasPrefix(lower, "set."):
		if v, ok := e.result.Value(lower[len("set."):]); ok {
			return []string{v}
		}
		return nil
	}

	if e.res == nil {
		return nil
	}
	return e.res.Values(field)
}

// render troca {campo} pelo valor (vários valores separados por ", ").
func (e evaluator) render(tpl string) string {
	if !strings.Contains(tpl, "{") {
		return tpl
	}
	return templateRe.ReplaceAllStringFunc(tpl, func(m string) string {
		vals := e.values(m[1 : len(m)-1])
		return strings.Join(nonEmpty(vals), ", ")
	})
}

func (e evaluator) evalIf(n config.FlowNode) (bool, string) {
	anyMode := n.Match == "any"
	result := !anyMode
	var parts []string

	for _, c := range n.Conditions {
		vals := e.values(c.Field)
		want := e.render(c.Value)
		ok := EvalCondition(vals, c.Op, want)

		desc := fmt.Sprintf("%s %s", c.Field, c.Op)
		if want != "" {
			desc += " " + want
		}
		mark := "✗"
		if ok {
			mark = "✓"
		}
		parts = append(parts, fmt.Sprintf("%s %s (%s)", mark, desc, showValues(vals)))

		if anyMode && ok {
			result = true
		}
		if !anyMode && !ok {
			result = false
		}
	}

	join := " E "
	if anyMode {
		join = " OU "
	}
	return result, strings.Join(parts, join)
}

// EvalCondition aplica o operador. Campo multivalorado: operador positivo
// vale se QUALQUER valor bater; o negativo (neq, not_*) se NENHUM bater.
func EvalCondition(vals []string, op, want string) bool {
	switch op {
	case "", "eq":
		return anyValue(vals, func(v string) bool { return strings.EqualFold(v, want) })
	case "neq":
		return !EvalCondition(vals, "eq", want)
	case "contains":
		w := strings.ToLower(want)
		return anyValue(vals, func(v string) bool { return strings.Contains(strings.ToLower(v), w) })
	case "not_contains":
		return !EvalCondition(vals, "contains", want)
	case "matches":
		return anyValue(vals, func(v string) bool { return Wildcard(want, v) })
	case "not_matches":
		return !EvalCondition(vals, "matches", want)
	case "in":
		list := splitList(want)
		return anyValue(vals, func(v string) bool {
			for _, w := range list {
				if Wildcard(w, v) {
					return true
				}
			}
			return false
		})
	case "not_in":
		return !EvalCondition(vals, "in", want)
	case "exists":
		return len(nonEmpty(vals)) > 0
	case "not_exists":
		return len(nonEmpty(vals)) == 0
	case "gt", "gte", "lt", "lte":
		w, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(want), ",", "."), 64)
		if err != nil {
			return false
		}
		return anyValue(vals, func(v string) bool {
			f, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(v), ",", "."), 64)
			if err != nil {
				return false
			}
			switch op {
			case "gt":
				return f > w
			case "gte":
				return f >= w
			case "lt":
				return f < w
			default:
				return f <= w
			}
		})
	case "is_true":
		return anyValue(vals, func(v string) bool { b, ok := config.ParseFlowBool(v); return ok && b })
	case "is_false":
		return !EvalCondition(vals, "is_true", want)
	}
	return false
}

// matchCase: o valor do caso aceita lista (vírgula) e curinga *.
func matchCase(vals []string, caseValue string) bool {
	return EvalCondition(vals, "in", caseValue)
}

// Wildcard compara sem diferenciar maiúsculas; * casa qualquer sequência.
// Sem * é igualdade.
func Wildcard(pattern, s string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	s = strings.ToLower(strings.TrimSpace(s))
	if !strings.Contains(pattern, "*") {
		return pattern == s
	}
	parts := strings.Split(pattern, "*")
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = s[len(parts[0]):]
	last := parts[len(parts)-1]
	for _, p := range parts[1 : len(parts)-1] {
		i := strings.Index(s, p)
		if i < 0 {
			return false
		}
		s = s[i+len(p):]
	}
	return strings.HasSuffix(s, last)
}

// InCalendar: dia da semana cadastrado, fora dos feriados e hora em
// [start, end).
func InCalendar(c config.Calendar, now time.Time) bool {
	date := now.Format("2006-01-02")
	for _, h := range c.Holidays {
		if strings.TrimSpace(h) == date {
			return false
		}
	}
	dayOK := false
	for _, d := range c.Days {
		if config.WeekdayIndex(d) == int(now.Weekday()) {
			dayOK = true
			break
		}
	}
	if !dayOK {
		return false
	}
	hm := now.Format("15:04")
	return hm >= c.Start && hm < c.End
}

func anyValue(vals []string, fn func(string) bool) bool {
	for _, v := range vals {
		if fn(v) {
			return true
		}
	}
	return false
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func nonEmpty(vals []string) []string {
	var out []string
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}

func showValues(vals []string) string {
	vals = nonEmpty(vals)
	if len(vals) == 0 {
		return "vazio"
	}
	return strings.Join(vals, ", ")
}
