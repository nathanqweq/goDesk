package app

import (
	"log"
	"strconv"
	"strings"
	"time"

	"godesk/internal/config"
	"godesk/internal/flow"
	"godesk/internal/metrics"
	"godesk/internal/rawdata"
	"godesk/internal/zabbix"
)

// Fluxos de decisão por rule (godesk-flows.yaml): a rule aponta pra um
// fluxo (flow) e ele roda depois de resolver a policy e checar se o
// chamado existe, antes de abrir/atualizar/fechar. O que o fluxo define
// (bloco "definir campos") sobrescreve o valor resolvido da rule —
// inclusive tag do Zabbix — e o bloco "fim" pode decidir não abrir o
// chamado (só comenta no evento).
//
// Fail-open, igual à dependência: arquivo inválido, fluxo inexistente ou
// erro no motor = a rule segue fixa, como se não tivesse fluxo.

// flowNow é o relógio dos fluxos (calendários, now.*) — substituível em
// teste.
var flowNow = time.Now

type flowZabbix interface {
	HostInfo(hostID, host string) (zabbix.HostInfo, error)
	EventTags(eventID string) ([]zabbix.Tag, error)
}

// eventResolver devolve os campos do evento pro motor. Tags e grupos do
// host e tags do evento só são buscados no Zabbix se o fluxo usar (uma
// chamada cada, no máximo).
type eventResolver struct {
	p            rawdata.Payload
	zx           flowZabbix
	ticketExists bool
	ticketStatus string

	hostLoaded bool
	host       zabbix.HostInfo
	evLoaded   bool
	evTags     []zabbix.Tag
}

func one(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return []string{v}
}

func (r *eventResolver) loadHost() {
	if r.hostLoaded {
		return
	}
	r.hostLoaded = true
	if r.zx == nil {
		return
	}
	h, err := r.zx.HostInfo(r.p.HostID, r.p.Host)
	if err != nil {
		log.Printf("[flow] WARN: falha ao buscar tags/grupos do host %q (campos vêm vazios): %v\n", r.p.Host, err)
		return
	}
	r.host = h
}

func (r *eventResolver) loadEvent() {
	if r.evLoaded {
		return
	}
	r.evLoaded = true
	if r.zx == nil {
		return
	}
	tags, err := r.zx.EventTags(r.p.EventID)
	if err != nil {
		log.Printf("[flow] WARN: falha ao buscar tags do evento %s (campos vêm vazios): %v\n", r.p.EventID, err)
		return
	}
	r.evTags = tags
}

func tagValues(tags []zabbix.Tag, name string) []string {
	var out []string
	for _, t := range tags {
		if strings.EqualFold(t.Tag, name) {
			out = append(out, t.Value)
		}
	}
	return out
}

// FlowFields: campos que o resolvedor conhece (o editor do módulo usa a
// mesma lista nas sugestões).
var FlowFields = []string{
	"host", "host.name", "host.id", "host.group", "host.tag.<nome>",
	"trigger", "trigger.id", "severity", "severity.level",
	"status", "event.kind", "event.id", "event.tag.<nome>", "value",
	"date", "hour", "rule", "cliente", "equipe",
	"ticket.exists", "ticket.status",
	"calendar.<nome>", "now.weekday", "now.hour", "now.time", "now.date", "set.<campo>",
}

func (r *eventResolver) Values(field string) []string {
	p := r.p
	lower := strings.ToLower(strings.TrimSpace(field))

	switch lower {
	case "host":
		return one(p.Host)
	case "host.id":
		return one(p.HostID)
	case "host.name":
		r.loadHost()
		return one(r.host.Name)
	case "host.group", "host.groups":
		r.loadHost()
		return r.host.Groups
	case "trigger":
		return one(p.Trigger)
	case "trigger.id":
		return one(p.TriggerID)
	case "severity":
		return one(p.Severity)
	case "severity.level":
		if lvl := severityLevel(p.Severity); lvl >= 0 {
			return []string{strconv.Itoa(lvl)}
		}
		return nil
	case "status":
		return one(p.Status)
	case "event.kind":
		if rawdata.EventKind(p) == "ProblemStart" {
			return []string{"problem"}
		}
		return []string{"recovery"}
	case "event.id":
		return one(p.EventID)
	case "value":
		return one(p.ValueItem)
	case "date":
		return one(p.Date)
	case "hour":
		return one(p.Hour)
	case "rule":
		return one(p.RuleName)
	case "cliente":
		return one(p.Cliente)
	case "equipe":
		return one(p.Equipe)
	case "ticket.exists":
		return []string{strconv.FormatBool(r.ticketExists)}
	case "ticket.status":
		return one(r.ticketStatus)
	}

	switch {
	case strings.HasPrefix(lower, "host.tag."):
		r.loadHost()
		return tagValues(r.host.Tags, field[len("host.tag."):])
	case strings.HasPrefix(lower, "event.tag."):
		r.loadEvent()
		return tagValues(r.evTags, field[len("event.tag."):])
	case strings.HasPrefix(lower, "tag."):
		r.loadEvent()
		return tagValues(r.evTags, field[len("tag."):])
	}
	return nil
}

// severityLevel: 0 (não classificada) .. 5 (desastre), nomes em inglês
// ou português, ou o número. -1 = desconhecida.
func severityLevel(s string) int {
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

// runFlow executa o fluxo da rule. Devolve nil quando a rule não tem
// fluxo, em shadow, ou em qualquer erro (a rule segue fixa).
func runFlow(cfg config.RuntimeConfig, zx flowZabbix, p rawdata.Payload, pol config.Policy, ticketExists bool, ticketStatus string, now time.Time) *flow.Result {
	name := strings.TrimSpace(pol.Flow)
	if name == "" {
		return nil
	}
	mode := config.NormalizeFlowMode(pol.FlowMode)

	fail := func(format string, args ...any) *flow.Result {
		recordMetric(cfg.MetricsFile, func(m *metrics.Snapshot) { m.FlowsErrors++ })
		log.Printf("[flow] WARN: "+format+" — seguindo a rule fixa\n", args...)
		return nil
	}

	f, err := config.LoadFlows(cfg.FlowsFile)
	if err != nil {
		return fail("falha ao ler %s: %v", cfg.FlowsFile, err)
	}
	fl, ok := f.Find(name)
	if !ok {
		return fail("rule=%q aponta pro fluxo %q, que não existe em %s", p.RuleName, name, cfg.FlowsFile)
	}

	res := &eventResolver{p: p, zx: zx, ticketExists: ticketExists, ticketStatus: ticketStatus}
	r, err := flow.Run(f, fl, res, now)
	if err != nil {
		return fail("fluxo %q: %v (caminho até o erro: %s)", name, err, r.Path())
	}

	sets := make([]string, 0, len(r.Set))
	for _, a := range r.Set {
		sets = append(sets, a.Field+"="+strconv.Quote(a.Value))
	}

	if mode == config.FlowModeShadow {
		recordMetric(cfg.MetricsFile, func(m *metrics.Snapshot) { m.FlowsShadowEvaluated++ })
		log.Printf("[flow-shadow] rule=%q fluxo=%q event=%s ação=%s caminho: %s | definiria: %s\n",
			p.RuleName, name, p.EventID, r.Action, r.Path(), strings.Join(sets, " "))
		return nil
	}

	recordMetric(cfg.MetricsFile, func(m *metrics.Snapshot) { m.FlowsEvaluated++ })
	log.Printf("[flow] rule=%q fluxo=%q event=%s ação=%s caminho: %s | definiu: %s\n",
		p.RuleName, name, p.EventID, r.Action, r.Path(), strings.Join(sets, " "))
	return &r
}

// applyFlow aplica os campos definidos pelo fluxo, na ordem. Texto vazio
// (template que não resolveu) não apaga o valor da rule.
func applyFlow(r flow.Result, pol *config.Policy, p *rawdata.Payload, fields map[string]*string) {
	for _, a := range r.Set {
		v := a.Value
		if config.FlowBoolFields[a.Field] {
			b, ok := config.ParseFlowBool(v)
			if !ok {
				continue
			}
			switch a.Field {
			case "autoclose":
				pol.AutoClose = b
			case "dependency":
				pol.Dependency = b
			case "custom_status":
				pol.CustomStatus = b
			case "send_teams":
				pol.TopDesk.SendTeams = b
			case "send_email":
				pol.TopDesk.SendEmail = b
			case "once_per_day":
				pol.TopDesk.OncePerDay = b
			case "send_more_info":
				pol.TopDesk.SendMoreInfo = b
			}
			continue
		}

		if strings.TrimSpace(v) == "" {
			continue
		}
		if ptr, ok := fields[a.Field]; ok {
			*ptr = v
			continue
		}
		switch a.Field {
		case "instrucao":
			p.Instrucao = v
		case "equipe":
			p.Equipe = v
		case "cliente":
			p.Cliente = v
		case "dependency_version":
			pol.DependencyVersion = v
		case "status_open":
			pol.StatusOpen = v
		case "status_update":
			pol.StatusUpdate = v
		case "teams_webhook":
			pol.TopDesk.TeamsWebhook = v
		case "email_to":
			pol.TopDesk.EmailTo = v
		case "email_cc":
			pol.TopDesk.EmailCc = v
		case "more_info_text":
			pol.TopDesk.MoreInfoText = v
		}
	}
}

var htmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

func htmlEscapeText(s string) string { return htmlEscaper.Replace(s) }

// flowSkipComment: comentário no evento quando o fluxo decide não abrir.
func flowSkipComment(r flow.Result) string {
	if c := strings.TrimSpace(r.Comment); c != "" {
		return c
	}
	return "goDesk (fluxo " + r.Flow + "): chamado não aberto — caminho: " + r.Path()
}
