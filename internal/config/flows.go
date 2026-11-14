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
//	table  → found | not_found       (primeira linha que bater define várias colunas)
//	set    → next                    (define campos: urgência, operador, instrução...)
//	update → next                    (chamado já aberto: muda campos/nota — escalonamento)
//	http   → next                    (chamada HTTP depois de tratar o chamado)
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

// Histórico das execuções e estado por evento (internal/flowlog).
const (
	DefaultFlowsHistoryFile = "/var/lib/godesk/godesk-flows-history.jsonl"
	DefaultFlowsStateFile   = "/var/lib/godesk/godesk-flows-state.json"
)

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
	NodeStart     = "start"
	NodeIf        = "if"
	NodeSwitch    = "switch"
	NodeSet       = "set"
	NodeEnd       = "end"
	NodeTable     = "table"
	NodeUpdate    = "update"
	NodeHTTP      = "http"
	NodeBranch    = "branch"    // se / senão se / senão
	NodeTransform = "transform" // cria variáveis {var.nome}
	NodeZabbix    = "zabbix"    // comentário/ack/severidade/supressão no evento
	NodeNote      = "note"      // texto na descrição ou ação no chamado
)

// Destinos do bloco note.
const (
	NoteDescription = "description" // descrição do chamado (abertura; visível ao solicitante)
	NoteInternal    = "internal"    // ação interna (invisível ao solicitante)
	NotePublic      = "public"      // ação visível ao solicitante
)

// SeverityLevel: 0 (não classificada) .. 5 (desastre) — nomes em inglês
// ou português, ou o número. -1 = desconhecida.
func SeverityLevel(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "0", "not classified", "não classificada", "nao classificada":
		return 0
	case "1", "information", "informação", "informacao":
		return 1
	case "2", "warning", "atenção", "atencao", "aviso":
		return 2
	case "3", "average", "média", "media":
		return 3
	case "4", "high", "alta":
		return 4
	case "5", "disaster", "desastre":
		return 5
	}
	return -1
}

// Ações do bloco end.
const (
	FlowActionOpen = "open" // segue o fluxo normal da rule (abre chamado)
	FlowActionSkip = "skip" // não abre chamado — só comenta no evento
)

// Portas fixas.
const (
	PortNext     = "next"
	PortTrue     = "true"
	PortFalse    = "false"
	PortDefault  = "default"
	PortFound    = "found"
	PortNotFound = "not_found"
	PortElse     = "else"
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
	"starts", "ends",
	"regex", "not_regex",
	"between",
}

// Operações do bloco transform.
var FlowTransformOps = []string{"set", "extract", "replace", "upper", "lower", "trim", "default"}

var varNameRe = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// betweenParts separa "a..b" (os dois lados obrigatórios).
func BetweenParts(v string) (lo, hi string, ok bool) {
	i := strings.Index(v, "..")
	if i < 0 {
		return "", "", false
	}
	lo, hi = strings.TrimSpace(v[:i]), strings.TrimSpace(v[i+2:])
	return lo, hi, lo != "" && hi != ""
}

// CompileFlowRegex: regex das condições e do transform, sempre sem
// diferenciar maiúsculas.
func CompileFlowRegex(p string) (*regexp.Regexp, error) {
	return regexp.Compile("(?i)" + p)
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
	"send_teams", "teams_webhook", "teams_title", "teams_message",
	"send_email", "email_to", "email_cc", "once_per_day",
	"send_more_info", "more_info_text",
	"repeat_update",
}

// FlowUpdateFields: campos que o bloco update (chamado já aberto) muda
// no TopDesk.
var FlowUpdateFields = []string{"urgency", "impact", "priority", "oper_group", "operator"}

// Métodos aceitos pelo bloco http.
var FlowHTTPMethods = []string{"POST", "GET", "PUT", "PATCH", "DELETE"}

// Filtros de template: {campo|json} (escapa pra dentro de string JSON) e
// {campo|url} (codifica pra URL).
var FlowTemplateFilters = []string{"json", "url"}

// FlowBoolFields: subconjunto booleano de FlowSettableFields.
var FlowBoolFields = map[string]bool{
	"autoclose": true, "dependency": true, "custom_status": true,
	"send_teams": true, "send_email": true, "once_per_day": true,
	"send_more_info": true, "repeat_update": true,
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

// FlowCondition é uma regra (campo + operador + valor) ou, com
// Conditions preenchido, um GRUPO de regras com o próprio Match (E/OU) —
// ex: (tag = Critico OU grupo = Core) E fora do horário.
type FlowCondition struct {
	Field string `yaml:"field,omitempty"`
	Op    string `yaml:"op,omitempty"`
	Value string `yaml:"value,omitempty"`

	Match      string          `yaml:"match,omitempty"`
	Conditions []FlowCondition `yaml:"conditions,omitempty"`
}

// IsGroup: a regra é um grupo de regras.
func (c FlowCondition) IsGroup() bool { return len(c.Conditions) > 0 }

// FlowBranch é um ramo da condição múltipla: ID é o nome da saída.
type FlowBranch struct {
	ID         string          `yaml:"id"`
	Label      string          `yaml:"label,omitempty"`
	Match      string          `yaml:"match,omitempty"`
	Conditions []FlowCondition `yaml:"conditions"`
}

// FlowTransform cria/atualiza a variável {var.<Var>} a partir de From
// (template): set (o próprio texto), extract (Arg = regex; 1º grupo ou o
// trecho todo), replace (troca Arg por Arg2), upper, lower, trim, default
// (From vazio → Arg).
type FlowTransform struct {
	Var  string `yaml:"var"`
	From string `yaml:"from"`
	Op   string `yaml:"op"`
	Arg  string `yaml:"arg,omitempty"`
	Arg2 string `yaml:"arg2,omitempty"`
}

type FlowCase struct {
	// Value aceita lista separada por vírgula e curinga * (ex: "Alta,Crit*").
	// O texto exato também é o nome da porta de saída do caso.
	Value string `yaml:"value"`
}

// FlowTableRow: Match aceita lista (vírgula) e curinga *; Values segue a
// ordem de FlowNode.Columns (célula vazia = não define).
type FlowTableRow struct {
	Match  string   `yaml:"match"`
	Values []string `yaml:"values"`
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

	// set (e update: campos de FlowUpdateFields)
	Set []FlowAssignment `yaml:"set,omitempty"`

	// table (usa Field como chave de busca)
	Columns []string       `yaml:"columns,omitempty"`
	Rows    []FlowTableRow `yaml:"rows,omitempty"`

	// update: nota (ação invisível pro solicitante), aviso no Teams e
	// repetir a cada reenvio (padrão: uma vez por evento)
	Note   string `yaml:"note,omitempty"`
	Teams  bool   `yaml:"teams,omitempty"`
	Repeat bool   `yaml:"repeat,omitempty"`

	// branch (se / senão se / senão)
	Branches []FlowBranch `yaml:"branches,omitempty"`

	// transform
	Transforms []FlowTransform `yaml:"transforms,omitempty"`

	// zabbix (ação no evento): Message = comentário (template), Ack,
	// Severity (vazio = não muda), SuppressHours (0 = não suprime). Repeat
	// vale aqui também (padrão: uma vez por evento).
	Message       string `yaml:"message,omitempty"`
	Ack           bool   `yaml:"ack,omitempty"`
	Severity      string `yaml:"severity,omitempty"`
	SuppressHours int    `yaml:"suppress_hours,omitempty"`

	// note (texto no chamado): Text (template) e Target (description |
	// internal | public). Repeat vale pras ações no reenvio.
	Text   string `yaml:"text,omitempty"`
	Target string `yaml:"target,omitempty"`

	// http
	Method  string   `yaml:"method,omitempty"`
	URL     string   `yaml:"url,omitempty"`
	Headers []string `yaml:"headers,omitempty"` // "Nome: valor"
	Body    string   `yaml:"body,omitempty"`
	Timeout int      `yaml:"timeout,omitempty"` // segundos (padrão 10, máx 30)

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
	case NodeStart, NodeSet, NodeUpdate, NodeHTTP, NodeTransform, NodeZabbix, NodeNote:
		return []string{PortNext}
	case NodeBranch:
		out := make([]string, 0, len(n.Branches)+1)
		for _, b := range n.Branches {
			out = append(out, b.ID)
		}
		return append(out, PortElse)
	case NodeTable:
		return []string{PortFound, PortNotFound}
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
			m, err := normalizeMatch(n.Match)
			if err != nil {
				return fmt.Errorf("bloco %q: %w", n.ID, err)
			}
			n.Match = m
			if len(n.Conditions) == 0 {
				return fmt.Errorf("bloco %q: condição sem regras", n.ID)
			}
			for j := range n.Conditions {
				if err := validateCondition(f, &n.Conditions[j], 1); err != nil {
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

		case NodeTable:
			if err := validateTable(f, n); err != nil {
				return err
			}

		case NodeBranch:
			if len(n.Branches) == 0 {
				return fmt.Errorf("bloco %q: condição múltipla sem ramos", n.ID)
			}
			seen := map[string]bool{PortElse: true, PortNext: true}
			for j := range n.Branches {
				b := &n.Branches[j]
				b.ID = strings.TrimSpace(b.ID)
				if b.ID == "" {
					return fmt.Errorf("bloco %q: ramo #%d sem id", n.ID, j+1)
				}
				if seen[strings.ToLower(b.ID)] {
					return fmt.Errorf("bloco %q: ramo %q repetido (ou reservado)", n.ID, b.ID)
				}
				seen[strings.ToLower(b.ID)] = true
				m, err := normalizeMatch(b.Match)
				if err != nil {
					return fmt.Errorf("bloco %q, ramo %q: %w", n.ID, b.ID, err)
				}
				b.Match = m
				if len(b.Conditions) == 0 {
					return fmt.Errorf("bloco %q: ramo %q sem regras", n.ID, b.ID)
				}
				for k := range b.Conditions {
					if err := validateCondition(f, &b.Conditions[k], 1); err != nil {
						return fmt.Errorf("bloco %q, ramo %q, regra #%d: %w", n.ID, b.ID, k+1, err)
					}
				}
			}

		case NodeZabbix:
			n.Severity = strings.TrimSpace(n.Severity)
			if strings.TrimSpace(n.Message) == "" && !n.Ack && n.Severity == "" && n.SuppressHours == 0 {
				return fmt.Errorf("bloco %q: ação no Zabbix sem nada pra fazer (comentário, ack, severidade ou supressão)", n.ID)
			}
			if n.Severity != "" && SeverityLevel(n.Severity) < 0 {
				return fmt.Errorf("bloco %q: severidade %q inválida (0 a 5 ou o nome)", n.ID, n.Severity)
			}
			if n.SuppressHours < 0 || n.SuppressHours > 720 {
				return fmt.Errorf("bloco %q: suprimir entre 1 e 720 horas", n.ID)
			}
			if err := validateTemplate(f, n.Message); err != nil {
				return fmt.Errorf("bloco %q, comentário: %w", n.ID, err)
			}

		case NodeNote:
			n.Target = strings.ToLower(strings.TrimSpace(n.Target))
			if n.Target == "" {
				n.Target = NoteInternal
			}
			if n.Target != NoteDescription && n.Target != NoteInternal && n.Target != NotePublic {
				return fmt.Errorf("bloco %q: destino %q inválido (description, internal, public)", n.ID, n.Target)
			}
			if strings.TrimSpace(n.Text) == "" {
				return fmt.Errorf("bloco %q: texto no chamado vazio", n.ID)
			}
			if err := validateTemplate(f, n.Text); err != nil {
				return fmt.Errorf("bloco %q: %w", n.ID, err)
			}

		case NodeTransform:
			if len(n.Transforms) == 0 {
				return fmt.Errorf("bloco %q: transformar sem operações", n.ID)
			}
			for j := range n.Transforms {
				if err := validateTransform(f, &n.Transforms[j]); err != nil {
					return fmt.Errorf("bloco %q, operação #%d: %w", n.ID, j+1, err)
				}
			}

		case NodeUpdate:
			if len(n.Set) == 0 && strings.TrimSpace(n.Note) == "" && !n.Teams {
				return fmt.Errorf("bloco %q: atualizar chamado sem nada pra fazer (campos, nota ou Teams)", n.ID)
			}
			for j := range n.Set {
				a := &n.Set[j]
				a.Field = strings.ToLower(strings.TrimSpace(a.Field))
				if !inList(FlowUpdateFields, a.Field) {
					return fmt.Errorf("bloco %q: campo %q não pode ser atualizado (válidos: %s)", n.ID, a.Field, strings.Join(FlowUpdateFields, ", "))
				}
				if err := validateTemplate(f, a.Value); err != nil {
					return fmt.Errorf("bloco %q, campo %s: %w", n.ID, a.Field, err)
				}
			}
			if err := validateTemplate(f, n.Note); err != nil {
				return fmt.Errorf("bloco %q, nota: %w", n.ID, err)
			}

		case NodeHTTP:
			if err := validateHTTP(f, n); err != nil {
				return err
			}

		default:
			return fmt.Errorf("bloco %q: tipo %q desconhecido (start, if, switch, table, set, update, http, end)", n.ID, n.Type)
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

func normalizeMatch(m string) (string, error) {
	m = strings.ToLower(strings.TrimSpace(m))
	if m == "" {
		m = "all"
	}
	if m != "all" && m != "any" {
		return "", fmt.Errorf("match deve ser all ou any")
	}
	return m, nil
}

// maxCondDepth: grupos dentro de grupos (3 níveis bastam e evitam abuso).
const maxCondDepth = 3

func validateCondition(f FlowsFile, c *FlowCondition, depth int) error {
	if c.IsGroup() {
		if depth > maxCondDepth {
			return fmt.Errorf("grupos aninhados demais (máx %d níveis)", maxCondDepth)
		}
		m, err := normalizeMatch(c.Match)
		if err != nil {
			return err
		}
		c.Match = m
		c.Field, c.Op, c.Value = "", "", ""
		for j := range c.Conditions {
			if err := validateCondition(f, &c.Conditions[j], depth+1); err != nil {
				return fmt.Errorf("grupo, regra #%d: %w", j+1, err)
			}
		}
		return nil
	}
	c.Match = ""
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
	// valor fixo (sem {variável}) dá pra conferir agora
	if !strings.Contains(c.Value, "{") {
		switch c.Op {
		case "regex", "not_regex":
			if _, err := CompileFlowRegex(c.Value); err != nil {
				return fmt.Errorf("regex inválida %q: %v", c.Value, err)
			}
		case "between":
			if _, _, ok := BetweenParts(c.Value); !ok {
				return fmt.Errorf("\"entre\" espera início..fim (ex: 30..60 ou 22:00..06:00), veio %q", c.Value)
			}
		}
	}
	return validateTemplate(f, c.Value)
}

func validateTransform(f FlowsFile, t *FlowTransform) error {
	t.Var = strings.TrimSpace(t.Var)
	t.Op = strings.ToLower(strings.TrimSpace(t.Op))
	if t.Op == "" {
		t.Op = "set"
	}
	if !varNameRe.MatchString(t.Var) {
		return fmt.Errorf("nome da variável %q inválido (letras, números e _)", t.Var)
	}
	if !inList(FlowTransformOps, t.Op) {
		return fmt.Errorf("operação %q inválida (%s)", t.Op, strings.Join(FlowTransformOps, ", "))
	}
	switch t.Op {
	case "extract":
		if _, err := CompileFlowRegex(t.Arg); err != nil || strings.TrimSpace(t.Arg) == "" {
			return fmt.Errorf("extrair precisa de uma regex válida, veio %q", t.Arg)
		}
	case "replace":
		if t.Arg == "" {
			return fmt.Errorf("substituir precisa do texto a procurar")
		}
	}
	for _, s := range []string{t.From, t.Arg2} {
		if err := validateTemplate(f, s); err != nil {
			return err
		}
	}
	if t.Op == "default" {
		return validateTemplate(f, t.Arg)
	}
	return nil
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

// TemplateFields devolve os campos citados em {campo} no texto (sem o
// filtro |json / |url).
func TemplateFields(s string) []string {
	var out []string
	for _, m := range templateVarRe.FindAllStringSubmatch(s, -1) {
		field, _ := SplitTemplateVar(m[1])
		out = append(out, field)
	}
	return out
}

// SplitTemplateVar separa "campo|filtro" (filtro vazio = nenhum).
func SplitTemplateVar(s string) (field, filter string) {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "|"); i >= 0 {
		return strings.TrimSpace(s[:i]), strings.ToLower(strings.TrimSpace(s[i+1:]))
	}
	return s, ""
}

func validateTemplate(f FlowsFile, s string) error {
	for _, m := range templateVarRe.FindAllStringSubmatch(s, -1) {
		field, filter := SplitTemplateVar(m[1])
		if filter != "" && !inList(FlowTemplateFilters, filter) {
			return fmt.Errorf("filtro %q inválido em {%s} (use |json ou |url)", filter, m[1])
		}
		if err := validateField(f, field); err != nil {
			return err
		}
	}
	return nil
}

func inList(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func validateTable(f FlowsFile, n *FlowNode) error {
	n.Field = strings.TrimSpace(n.Field)
	if n.Field == "" {
		return fmt.Errorf("bloco %q: tabela sem campo de busca", n.ID)
	}
	if err := validateField(f, n.Field); err != nil {
		return fmt.Errorf("bloco %q: %w", n.ID, err)
	}
	if len(n.Columns) == 0 {
		return fmt.Errorf("bloco %q: tabela sem colunas", n.ID)
	}
	seen := map[string]bool{}
	for j := range n.Columns {
		c := strings.ToLower(strings.TrimSpace(n.Columns[j]))
		n.Columns[j] = c
		if !isSettable(c) {
			return fmt.Errorf("bloco %q: coluna %q não pode ser definida", n.ID, c)
		}
		if seen[c] {
			return fmt.Errorf("bloco %q: coluna %q repetida", n.ID, c)
		}
		seen[c] = true
	}
	if len(n.Rows) == 0 {
		return fmt.Errorf("bloco %q: tabela sem linhas", n.ID)
	}
	for j := range n.Rows {
		row := &n.Rows[j]
		row.Match = strings.TrimSpace(row.Match)
		if row.Match == "" {
			return fmt.Errorf("bloco %q: linha #%d sem valor de busca", n.ID, j+1)
		}
		if len(row.Values) > len(n.Columns) {
			return fmt.Errorf("bloco %q: linha #%d tem mais valores que colunas", n.ID, j+1)
		}
		for k, v := range row.Values {
			col := n.Columns[k]
			if FlowBoolFields[col] && strings.TrimSpace(v) != "" {
				if _, ok := ParseFlowBool(v); !ok {
					return fmt.Errorf("bloco %q: linha #%d, %s espera true/false, veio %q", n.ID, j+1, col, v)
				}
			}
			if err := validateTemplate(f, v); err != nil {
				return fmt.Errorf("bloco %q, linha #%d: %w", n.ID, j+1, err)
			}
		}
	}
	return nil
}

func validateHTTP(f FlowsFile, n *FlowNode) error {
	n.Method = strings.ToUpper(strings.TrimSpace(n.Method))
	if n.Method == "" {
		n.Method = "POST"
	}
	if !inList(FlowHTTPMethods, n.Method) {
		return fmt.Errorf("bloco %q: método %q inválido (%s)", n.ID, n.Method, strings.Join(FlowHTTPMethods, ", "))
	}
	n.URL = strings.TrimSpace(n.URL)
	lu := strings.ToLower(n.URL)
	if !strings.HasPrefix(lu, "http://") && !strings.HasPrefix(lu, "https://") {
		return fmt.Errorf("bloco %q: a URL precisa começar com http:// ou https://", n.ID)
	}
	if n.Timeout < 0 || n.Timeout > 30 {
		return fmt.Errorf("bloco %q: timeout entre 1 e 30 segundos", n.ID)
	}
	for j, h := range n.Headers {
		if i := strings.Index(h, ":"); i <= 0 || strings.TrimSpace(h[:i]) == "" {
			return fmt.Errorf("bloco %q: cabeçalho #%d no formato \"Nome: valor\"", n.ID, j+1)
		}
		if err := validateTemplate(f, h); err != nil {
			return fmt.Errorf("bloco %q, cabeçalho #%d: %w", n.ID, j+1, err)
		}
	}
	for _, s := range []string{n.URL, n.Body} {
		if err := validateTemplate(f, s); err != nil {
			return fmt.Errorf("bloco %q: %w", n.ID, err)
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
