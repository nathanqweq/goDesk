package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// godesk-flows.yaml — fluxos de decisão das rules. Em vez de criar uma
// rule nova pra cada variação ("este host é crítico", "fora do horário
// liga", "esta mensagem é outra"), a rule aponta pra um fluxo (campo flow
// no godesk-config.yaml) e o fluxo decide, na hora do alerta, olhando os
// dados do evento (severidade, tags e grupos do host, tags do evento,
// horário...).
//
// Um fluxo é um grafo de blocos ligados por portas:
//
//	start  → next
//	if     → true | false            (condições com E/OU)
//	switch → <valor de cada caso> | default
//	set    → next                    (define campos: urgência, operador, instrução...)
//	end    → —                       (abrir chamado ou não abrir)
//
// Porta sem ligação = fim com "abrir" (o fluxo de sempre da rule, com o
// que já tiver sido definido). Ciclos são recusados na validação.
//
//	calendars:
//	  comercial:
//	    days: [seg, ter, qua, qui, sex]
//	    start: "08:00"
//	    end: "18:00"
//	    holidays: ["2026-12-25"]
//	flows:
//	  - name: infra
//	    nodes:
//	      - {id: inicio, type: start}
//	      - id: crit
//	        type: switch
//	        field: host.tag.Criticidade
//	        cases: [{value: Critico}]
//	      - id: ligar
//	        type: set
//	        set: [{field: instrucao, value: "Ligar para o cliente ({host})"}]
//	    links:
//	      - {from: inicio, to: crit}
//	      - {from: crit, port: Critico, to: ligar}

const DefaultFlowsFile = "/etc/zabbix/godesk/godesk-flows.yaml"

// Modos do fluxo por rule (Policy.FlowMode).
const (
	FlowModeOn     = "on"
	FlowModeShadow = "shadow"
)

// NormalizeFlowMode: vazio ou desconhecido = on (a rule só tem flow se
// alguém escolheu um fluxo pra ela).
func NormalizeFlowMode(v string) string {
	if strings.EqualFold(strings.TrimSpace(v), FlowModeShadow) {
		return FlowModeShadow
	}
	return FlowModeOn
}

// Tipos de bloco.
const (
	NodeStart  = "start"
	NodeIf     = "if"
	NodeSwitch = "switch"
	NodeSet    = "set"
	NodeEnd    = "end"
)

// Ações do bloco end.
const (
	FlowActionOpen = "open" // segue o fluxo normal da rule (abre chamado)
	FlowActionSkip = "skip" // não abre chamado — só comenta no evento
)

// Portas fixas.
const (
	PortNext    = "next"
	PortTrue    = "true"
	PortFalse   = "false"
	PortDefault = "default"
)

// Operadores das condições.
var FlowOps = []string{
	"eq", "neq",
	"contains", "not_contains",
	"matches", "not_matches",
	"in", "not_in",
	"exists", "not_exists",
	"gt", "gte", "lt", "lte",
	"is_true", "is_false",
}

// FlowSettableFields: campos que um bloco set pode definir. Os de texto
// sobrescrevem o valor resolvido da rule (inclusive tag do Zabbix); os
// booleanos aceitam true/false/sim/não/1/0.
var FlowSettableFields = []string{
	"instrucao", "equipe", "cliente",
	"contract", "operator", "oper_group", "main_caller", "secundary_caller",
	"sla", "category", "sub_category", "call_type",
	"urgency", "impact", "priority",
	"autoclose", "dependency", "dependency_version",
	"custom_status", "status_open", "status_update",
	"send_teams", "teams_webhook",
	"send_email", "email_to", "email_cc", "once_per_day",
	"send_more_info", "more_info_text",
}

// FlowBoolFields: subconjunto booleano de FlowSettableFields.
var FlowBoolFields = map[string]bool{
	"autoclose": true, "dependency": true, "custom_status": true,
	"send_teams": true, "send_email": true, "once_per_day": true,
	"send_more_info": true,
}

var weekdayNames = map[string]int{
	"dom": 0, "seg": 1, "ter": 2, "qua": 3, "qui": 4, "sex": 5, "sab": 6, "sáb": 6,
	"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6,
}

// WeekdayIndex devolve 0 (domingo) .. 6 (sábado) ou -1.
func WeekdayIndex(s string) int {
	if i, ok := weekdayNames[strings.ToLower(strings.TrimSpace(s))]; ok {
		return i
	}
	return -1
}

var hhmmRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)
var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

type Calendar struct {
	Days     []string `yaml:"days"`
	Start    string   `yaml:"start"`
	End      string   `yaml:"end"`
	Holidays []string `yaml:"holidays,omitempty"`
}

type FlowCondition struct {
	Field string `yaml:"field"`
	Op    string `yaml:"op"`
	Value string `yaml:"value,omitempty"`
}

type FlowCase struct {
	// Value aceita lista separada por vírgula e curinga * (ex: "Alta,Crit*").
	// O texto exato também é o nome da porta de saída do caso.
	Value string `yaml:"value"`
}

type FlowAssignment struct {
	Field string `yaml:"field"`
	// Value é um template: {host}, {severity}, {host.tag.Site}...
	Value string `yaml:"value"`
}

type FlowNode struct {
	ID    string `yaml:"id"`
	Type  string `yaml:"type"`
	Label string `yaml:"label,omitempty"`
	X     int    `yaml:"x,omitempty"`
	Y     int    `yaml:"y,omitempty"`

	// if
	Match      string          `yaml:"match,omitempty"` // all (padrão) | any
	Conditions []FlowCondition `yaml:"conditions,omitempty"`

	// switch
	Field string     `yaml:"field,omitempty"`
	Cases []FlowCase `yaml:"cases,omitempty"`

	// set
	Set []FlowAssignment `yaml:"set,omitempty"`

	// end
	Action  string `yaml:"action,omitempty"`  // open (padrão) | skip
	Comment string `yaml:"comment,omitempty"` // comentário no evento quando skip
}

type FlowLink struct {
	From string `yaml:"from"`
	Port string `yaml:"port,omitempty"` // vazio = next
	To   string `yaml:"to"`
}

type Flow struct {
	Name        string     `yaml:"name"`
	Description string     `yaml:"description,omitempty"`
	Nodes       []FlowNode `yaml:"nodes"`
	Links       []FlowLink `yaml:"links"`
}

type FlowsFile struct {
	Calendars map[string]Calendar `yaml:"calendars"`
	Flows     []Flow              `yaml:"flows"`
}

// Find devolve o fluxo pelo nome (case-insensitive).
func (f FlowsFile) Find(name string) (Flow, bool) {
	name = strings.TrimSpace(name)
	for _, fl := range f.Flows {
		if strings.EqualFold(fl.Name, name) {
			return fl, true
		}
	}
	return Flow{}, false
}

// Calendar devolve o calendário pelo nome (case-insensitive).
func (f FlowsFile) Calendar(name string) (Calendar, bool) {
	if c, ok := f.Calendars[name]; ok {
		return c, true
	}
	for k, c := range f.Calendars {
		if strings.EqualFold(k, name) {
			return c, true
		}
	}
	return Calendar{}, false
}

func (fl Flow) Node(id string) (FlowNode, bool) {
	for _, n := range fl.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return FlowNode{}, false
}

// Next devolve o id do bloco ligado na porta (vazio = sem ligação).
func (fl Flow) Next(from, port string) string {
	for _, l := range fl.Links {
		if l.From == from && linkPort(l) == port {
			return l.To
		}
	}
	return ""
}

func (fl Flow) Start() (FlowNode, bool) {
	for _, n := range fl.Nodes {
		if n.Type == NodeStart {
			return n, true
		}
	}
	return FlowNode{}, false
}

func linkPort(l FlowLink) string {
	if l.Port == "" {
		return PortNext
	}
	return l.Port
}

// Ports devolve as portas de saída válidas do bloco.
func (n FlowNode) Ports() []string {
	switch n.Type {
	case NodeStart, NodeSet:
		return []string{PortNext}
	case NodeIf:
		return []string{PortTrue, PortFalse}
	case NodeSwitch:
		out := make([]string, 0, len(n.Cases)+1)
		for _, c := range n.Cases {
			out = append(out, c.Value)
		}
		return append(out, PortDefault)
	default:
		return nil
	}
}

func LoadFlows(path string) (FlowsFile, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return FlowsFile{}, nil
	}
	if err != nil {
		return FlowsFile{}, err
	}
	return ParseFlows(b)
}

// ParseFlows faz o parse, normaliza (trim, defaults) e valida tudo que
// dá pra validar sem o Zabbix: nomes únicos, um start por fluxo, tipos,
// portas, ligações, ciclos, operadores, campos do set e calendários.
func ParseFlows(data []byte) (FlowsFile, error) {
	var f FlowsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return FlowsFile{}, err
	}

	for name, c := range f.Calendars {
		if strings.TrimSpace(name) == "" {
			return FlowsFile{}, fmt.Errorf("calendário sem nome")
		}
		if len(c.Days) == 0 {
			return FlowsFile{}, fmt.Errorf("calendário %q: informe os dias (seg, ter, ...)", name)
		}
		for _, d := range c.Days {
			if WeekdayIndex(d) < 0 {
				return FlowsFile{}, fmt.Errorf("calendário %q: dia %q inválido (use dom, seg, ter, qua, qui, sex, sab)", name, d)
			}
		}
		if !hhmmRe.MatchString(c.Start) || !hhmmRe.MatchString(c.End) {
			return FlowsFile{}, fmt.Errorf("calendário %q: início/fim no formato HH:MM", name)
		}
		if c.Start >= c.End {
			return FlowsFile{}, fmt.Errorf("calendário %q: início (%s) precisa ser antes do fim (%s)", name, c.Start, c.End)
		}
		for _, h := range c.Holidays {
			if !dateRe.MatchString(strings.TrimSpace(h)) {
				return FlowsFile{}, fmt.Errorf("calendário %q: feriado %q no formato AAAA-MM-DD", name, h)
			}
		}
	}

	names := map[string]bool{}
	for i := range f.Flows {
		fl := &f.Flows[i]
		fl.Name = strings.TrimSpace(fl.Name)
		if fl.Name == "" {
			return FlowsFile{}, fmt.Errorf("flows[%d]: nome do fluxo vazio", i)
		}
		if names[strings.ToLower(fl.Name)] {
			return FlowsFile{}, fmt.Errorf("fluxo %q cadastrado mais de uma vez", fl.Name)
		}
		names[strings.ToLower(fl.Name)] = true

		if err := validateFlow(f, fl); err != nil {
			return FlowsFile{}, fmt.Errorf("fluxo %q: %w", fl.Name, err)
		}
	}

	return f, nil
}

func validateFlow(f FlowsFile, fl *Flow) error {
	ids := map[string]*FlowNode{}
	starts := 0

	for i := range fl.Nodes {
		n := &fl.Nodes[i]
		n.ID = strings.TrimSpace(n.ID)
		n.Type = strings.ToLower(strings.TrimSpace(n.Type))
		if n.ID == "" {
			return fmt.Errorf("bloco #%d sem id", i+1)
		}
		if ids[n.ID] != nil {
			return fmt.Errorf("bloco %q repetido", n.ID)
		}
		ids[n.ID] = n

		switch n.Type {
		case NodeStart:
			starts++

		case NodeIf:
			n.Match = strings.ToLower(strings.TrimSpace(n.Match))
			if n.Match == "" {
				n.Match = "all"
			}
			if n.Match != "all" && n.Match != "any" {
				return fmt.Errorf("bloco %q: match deve ser all ou any", n.ID)
			}
			if len(n.Conditions) == 0 {
				return fmt.Errorf("bloco %q: condição sem regras", n.ID)
			}
			for j := range n.Conditions {
				if err := validateCondition(f, &n.Conditions[j]); err != nil {
					return fmt.Errorf("bloco %q, regra #%d: %w", n.ID, j+1, err)
				}
			}

		case NodeSwitch:
			n.Field = strings.TrimSpace(n.Field)
			if n.Field == "" {
				return fmt.Errorf("bloco %q: switch sem campo", n.ID)
			}
			if err := validateField(f, n.Field); err != nil {
				return fmt.Errorf("bloco %q: %w", n.ID, err)
			}
			if len(n.Cases) == 0 {
				return fmt.Errorf("bloco %q: switch sem casos", n.ID)
			}
			seen := map[string]bool{PortDefault: true}
			for j := range n.Cases {
				c := &n.Cases[j]
				c.Value = strings.TrimSpace(c.Value)
				if c.Value == "" {
					return fmt.Errorf("bloco %q: caso #%d vazio", n.ID, j+1)
				}
				if seen[strings.ToLower(c.Value)] {
					return fmt.Errorf("bloco %q: caso %q repetido (ou igual a default)", n.ID, c.Value)
				}
				seen[strings.ToLower(c.Value)] = true
			}

		case NodeSet:
			if len(n.Set) == 0 {
				return fmt.Errorf("bloco %q: definir campos sem nenhum campo", n.ID)
			}
			for j := range n.Set {
				a := &n.Set[j]
				a.Field = strings.ToLower(strings.TrimSpace(a.Field))
				if !isSettable(a.Field) {
					return fmt.Errorf("bloco %q: campo %q não pode ser definido (válidos: %s)", n.ID, a.Field, strings.Join(FlowSettableFields, ", "))
				}
				if FlowBoolFields[a.Field] {
					if _, ok := ParseFlowBool(a.Value); !ok {
						return fmt.Errorf("bloco %q: %s espera true/false, veio %q", n.ID, a.Field, a.Value)
					}
				}
				if err := validateTemplate(f, a.Value); err != nil {
					return fmt.Errorf("bloco %q, campo %s: %w", n.ID, a.Field, err)
				}
			}

		case NodeEnd:
			n.Action = strings.ToLower(strings.TrimSpace(n.Action))
			if n.Action == "" {
				n.Action = FlowActionOpen
			}
			if n.Action != FlowActionOpen && n.Action != FlowActionSkip {
				return fmt.Errorf("bloco %q: ação deve ser open ou skip", n.ID)
			}
			if err := validateTemplate(f, n.Comment); err != nil {
				return fmt.Errorf("bloco %q, comentário: %w", n.ID, err)
			}

		default:
			return fmt.Errorf("bloco %q: tipo %q desconhecido (start, if, switch, set, end)", n.ID, n.Type)
		}
	}

	if starts != 1 {
		return fmt.Errorf("precisa de exatamente 1 bloco de início (tem %d)", starts)
	}

	used := map[string]bool{}
	for i := range fl.Links {
		l := &fl.Links[i]
		l.From, l.To, l.Port = strings.TrimSpace(l.From), strings.TrimSpace(l.To), strings.TrimSpace(l.Port)
		from, to := ids[l.From], ids[l.To]
		if from == nil {
			return fmt.Errorf("ligação #%d sai de bloco inexistente %q", i+1, l.From)
		}
		if to == nil {
			return fmt.Errorf("ligação #%d chega em bloco inexistente %q", i+1, l.To)
		}
		if to.Type == NodeStart {
			return fmt.Errorf("ligação #%d chega no início", i+1)
		}
		if l.From == l.To {
			return fmt.Errorf("ligação #%d liga %q nele mesmo", i+1, l.From)
		}
		// porta do switch: aceita sem diferenciar maiúsculas, grava como no caso
		port := linkPort(*l)
		valid := ""
		for _, p := range from.Ports() {
			if strings.EqualFold(p, port) {
				valid = p
				break
			}
		}
		if valid == "" {
			return fmt.Errorf("ligação #%d: bloco %q (%s) não tem a saída %q", i+1, l.From, from.Type, port)
		}
		if valid == PortNext {
			l.Port = ""
		} else {
			l.Port = valid
		}
		key := l.From + "\x00" + valid
		if used[key] {
			return fmt.Errorf("saída %q do bloco %q ligada mais de uma vez", valid, l.From)
		}
		used[key] = true
	}

	// ciclo (DFS): o motor também limita passos, mas melhor recusar antes
	state := map[string]int{} // 0 novo, 1 na pilha, 2 visitado
	var visit func(id string) error
	visit = func(id string) error {
		switch state[id] {
		case 1:
			return fmt.Errorf("ligações formam um ciclo passando por %q", id)
		case 2:
			return nil
		}
		state[id] = 1
		for _, l := range fl.Links {
			if l.From == id {
				if err := visit(l.To); err != nil {
					return err
				}
			}
		}
		state[id] = 2
		return nil
	}
	for _, n := range fl.Nodes {
		if err := visit(n.ID); err != nil {
			return err
		}
	}

	return nil
}

func validateCondition(f FlowsFile, c *FlowCondition) error {
	c.Field = strings.TrimSpace(c.Field)
	c.Op = strings.ToLower(strings.TrimSpace(c.Op))
	if c.Field == "" {
		return fmt.Errorf("campo vazio")
	}
	if err := validateField(f, c.Field); err != nil {
		return err
	}
	if c.Op == "" {
		c.Op = "eq"
	}
	ok := false
	for _, op := range FlowOps {
		if op == c.Op {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("operador %q inválido (válidos: %s)", c.Op, strings.Join(FlowOps, ", "))
	}
	return validateTemplate(f, c.Value)
}

// validateField só confere o que é do próprio arquivo: calendário citado
// tem que existir. Os demais campos (host, host.tag.X, ...) são resolvidos
// na hora — campo desconhecido simplesmente vem vazio.
func validateField(f FlowsFile, field string) error {
	if strings.HasPrefix(strings.ToLower(field), "calendar.") {
		name := field[len("calendar."):]
		if _, ok := f.Calendar(name); !ok {
			return fmt.Errorf("calendário %q não cadastrado", name)
		}
	}
	return nil
}

var templateVarRe = regexp.MustCompile(`\{([^{}]+)\}`)

// TemplateFields devolve os campos citados em {campo} no texto.
func TemplateFields(s string) []string {
	var out []string
	for _, m := range templateVarRe.FindAllStringSubmatch(s, -1) {
		out = append(out, strings.TrimSpace(m[1]))
	}
	return out
}

func validateTemplate(f FlowsFile, s string) error {
	for _, field := range TemplateFields(s) {
		if err := validateField(f, field); err != nil {
			return err
		}
	}
	return nil
}

func isSettable(field string) bool {
	for _, s := range FlowSettableFields {
		if s == field {
			return true
		}
	}
	return false
}

// ParseFlowBool: true/false, sim/não, 1/0, yes/no, on/off.
func ParseFlowBool(v string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "sim", "1", "yes", "on":
		return true, true
	case "false", "não", "nao", "0", "no", "off":
		return false, true
	}
	return false, false
}
