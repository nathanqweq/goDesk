package app

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"godesk/internal/config"
	"godesk/internal/flow"
	"godesk/internal/flowlog"
	"godesk/internal/metrics"
	"godesk/internal/rawdata"
	"godesk/internal/zabbix"
)

// Fluxos de decisão por rule (godesk-flows.yaml): a rule aponta pra um
// fluxo (flow) e ele roda depois de resolver a policy e checar se o
// chamado existe, antes de abrir/atualizar/fechar. O que o fluxo define
// (blocos "definir campos" e "tabela") sobrescreve o valor resolvido da
// rule — inclusive tag do Zabbix —, o bloco "fim" pode decidir não abrir
// o chamado (só comenta no evento), o "atualizar chamado" escala um
// chamado já aberto quando o alerta é reenviado e o "http" chama outro
// sistema depois de tratar o chamado.
//
// Toda execução (ativa, shadow ou com erro) vai pro histórico
// (flowlog), lido pela página Fluxos → Histórico.
//
// Fail-open, igual à dependência: arquivo inválido, fluxo inexistente ou
// erro no motor = a rule segue fixa, como se não tivesse fluxo.

// flowNow é o relógio dos fluxos (calendários, now.*, event.age) —
// substituível em teste.
var flowNow = time.Now

type flowZabbix interface {
	HostInfo(hostID, host string) (zabbix.HostInfo, error)
	EventInfo(eventID string) (zabbix.EventInfo, error)
}

// ticketNumberPlaceholder: no momento do fluxo o chamado pode ainda não
// existir; o bloco http recebe o texto literal e o app troca depois.
const ticketNumberPlaceholder = "{ticket.number}"

// eventResolver devolve os campos do evento pro motor. Tags e grupos do
// host e dados do evento só são buscados no Zabbix se o fluxo usar (uma
// chamada cada, no máximo).
type eventResolver struct {
	p            rawdata.Payload
	zx           flowZabbix
	now          time.Time
	ticketExists bool
	ticketStatus string
	ticketID     string
	resends      int

	hostLoaded bool
	host       zabbix.HostInfo
	evLoaded   bool
	ev         zabbix.EventInfo
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
	ev, err := r.zx.EventInfo(r.p.EventID)
	if err != nil {
		log.Printf("[flow] WARN: falha ao buscar dados do evento %s (campos vêm vazios): %v\n", r.p.EventID, err)
		return
	}
	r.ev = ev
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

// eventStart: início do problema pelo Zabbix (event.get) ou, sem ele, pela
// data/hora do RAWDATA ({EVENT.DATE} {EVENT.TIME}).
func (r *eventResolver) eventStart() (time.Time, bool) {
	r.loadEvent()
	if !r.ev.Clock.IsZero() {
		return r.ev.Clock, true
	}
	for _, layout := range []string{"2006.01.02 15:04:05", "2006-01-02 15:04:05", "02/01/2006 15:04:05"} {
		if t, err := time.ParseInLocation(layout, strings.TrimSpace(r.p.Date)+" "+strings.TrimSpace(r.p.Hour), time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// FlowFields: campos que o resolvedor conhece (o editor do módulo usa a
// mesma lista nas sugestões).
var FlowFields = []string{
	"host", "host.name", "host.id", "host.group", "host.tag.<nome>",
	"trigger", "trigger.id", "severity", "severity.level",
	"status", "event.kind", "event.id", "event.tag.<nome>", "value",
	"event.age", "event.resends", "event.acknowledged", "event.acked_by_user",
	"date", "hour", "rule", "cliente", "equipe",
	"ticket.exists", "ticket.status", "ticket.number",
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
	case "event.age":
		if start, ok := r.eventStart(); ok {
			min := int(r.now.Sub(start).Minutes())
			if min < 0 {
				min = 0
			}
			return []string{strconv.Itoa(min)}
		}
		return nil
	case "event.resends":
		return []string{strconv.Itoa(r.resends)}
	case "event.acknowledged":
		r.loadEvent()
		return []string{strconv.FormatBool(r.ev.Acknowledged)}
	case "event.acked_by_user":
		r.loadEvent()
		return []string{strconv.FormatBool(r.ev.AckedByUser)}
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
	case "ticket.number":
		if r.ticketID != "" {
			return []string{r.ticketID}
		}
		return []string{ticketNumberPlaceholder}
	}

	switch {
	case strings.HasPrefix(lower, "host.tag."):
		r.loadHost()
		return tagValues(r.host.Tags, field[len("host.tag."):])
	case strings.HasPrefix(lower, "event.tag."):
		r.loadEvent()
		return tagValues(r.ev.Tags, field[len("event.tag."):])
	case strings.HasPrefix(lower, "tag."):
		r.loadEvent()
		return tagValues(r.ev.Tags, field[len("tag."):])
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

// flowRun é uma execução de fluxo dentro de um alerta: o resultado, se
// foi aplicado (modo ativo, sem erro) e o que o app fez com ele — vira uma
// linha do histórico no fim do alerta (record).
type flowRun struct {
	res     flow.Result
	mode    string
	applied bool
	done    map[string]bool // updates já aplicados neste evento
	entry   flowlog.Entry
}

func (fr *flowRun) note(format string, args ...any) {
	fr.entry.Notes = append(fr.entry.Notes, fmt.Sprintf(format, args...))
}

// runFlow executa o fluxo da rule. nil = rule sem fluxo. Em shadow ou erro
// devolve applied=false (a rule segue fixa) — mas registra no histórico.
func runFlow(cfg config.RuntimeConfig, zx flowZabbix, p rawdata.Payload, pol config.Policy, ticketExists bool, ticketStatus, ticketID string, now time.Time) *flowRun {
	name := strings.TrimSpace(pol.Flow)
	if name == "" {
		return nil
	}
	mode := config.NormalizeFlowMode(pol.FlowMode)
	kind := "recovery"
	if rawdata.EventKind(p) == "ProblemStart" {
		kind = "problem"
	}
	fr := &flowRun{mode: mode, done: map[string]bool{}, entry: flowlog.Entry{
		Time: now, EventID: p.EventID, Host: p.Host, Trigger: p.Trigger, Rule: p.RuleName,
		Flow: name, Mode: mode, Kind: kind, Ticket: ticketID,
	}}

	fail := func(format string, args ...any) *flowRun {
		recordMetric(cfg.MetricsFile, func(m *metrics.Snapshot) { m.FlowsErrors++ })
		msg := fmt.Sprintf(format, args...)
		log.Printf("[flow] WARN: %s — seguindo a rule fixa\n", msg)
		fr.entry.Error = msg
		fr.entry.Action = "rule fixa"
		return fr
	}

	f, err := config.LoadFlows(cfg.FlowsFile)
	if err != nil {
		return fail("falha ao ler %s: %v", cfg.FlowsFile, err)
	}
	fl, ok := f.Find(name)
	if !ok {
		return fail("rule=%q aponta pro fluxo %q, que não existe em %s", p.RuleName, name, cfg.FlowsFile)
	}

	prevRuns, done, err := flowlog.Touch(cfg.FlowsStateFile, p.EventID, now)
	if err != nil {
		log.Printf("[flow] WARN: falha ao gravar estado do evento (event.resends/updates): %v\n", err)
	}
	for _, d := range done {
		fr.done[d] = true
	}

	res := &eventResolver{p: p, zx: zx, now: now, ticketExists: ticketExists, ticketStatus: ticketStatus, ticketID: ticketID, resends: prevRuns}
	r, err := flow.Run(f, fl, res, now)
	fr.res = r
	fr.entry.Trace, fr.entry.Path = r.Trace, r.Path()
	if err != nil {
		return fail("fluxo %q: %v (caminho até o erro: %s)", name, err, r.Path())
	}
	fr.entry.Action, fr.entry.Set, fr.entry.Updates, fr.entry.HTTP, fr.entry.Vars = r.Action, r.Set, r.Updates, r.HTTP, r.Vars

	sets := make([]string, 0, len(r.Set))
	for _, a := range r.Set {
		sets = append(sets, a.Field+"="+strconv.Quote(a.Value))
	}

	if mode == config.FlowModeShadow {
		recordMetric(cfg.MetricsFile, func(m *metrics.Snapshot) { m.FlowsShadowEvaluated++ })
		log.Printf("[flow-shadow] rule=%q fluxo=%q event=%s ação=%s caminho: %s | definiria: %s\n",
			p.RuleName, name, p.EventID, r.Action, r.Path(), strings.Join(sets, " "))
		return fr
	}

	recordMetric(cfg.MetricsFile, func(m *metrics.Snapshot) { m.FlowsEvaluated++ })
	log.Printf("[flow] rule=%q fluxo=%q event=%s ação=%s caminho: %s | definiu: %s\n",
		p.RuleName, name, p.EventID, r.Action, r.Path(), strings.Join(sets, " "))
	fr.applied = true
	return fr
}

// flowExtras: campos do fluxo que não são da policy.
type flowExtras struct {
	repeatUpdate bool // "Recebemos novamente o alerta..." no reenvio (padrão true)
}

// applyFlow aplica os campos definidos pelo fluxo, na ordem. Texto vazio
// (template que não resolveu) não apaga o valor da rule.
func applyFlow(r flow.Result, pol *config.Policy, p *rawdata.Payload, fields map[string]*string) flowExtras {
	ex := flowExtras{repeatUpdate: true}
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
			case "repeat_update":
				ex.repeatUpdate = b
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
		case "teams_title":
			p.TeamsTitle = v
		case "teams_message":
			p.TeamsMessage = v
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
	return ex
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

// updatePatch monta o PATCH do TopDesk de um bloco update (campos + nota
// invisível pro solicitante). Vazio = nada a mudar no chamado.
func updatePatch(u flow.Update) map[string]any {
	patch := map[string]any{}
	for _, a := range u.Fields {
		v := strings.TrimSpace(a.Value)
		if v == "" {
			continue
		}
		switch a.Field {
		case "urgency":
			patch["urgency"] = map[string]any{"name": v}
		case "impact":
			patch["impact"] = map[string]any{"name": v}
		case "priority":
			patch["priority"] = map[string]any{"name": v}
		case "oper_group":
			patch["operatorGroup"] = map[string]any{"id": v}
		case "operator":
			patch["operator"] = map[string]any{"id": v}
		}
	}
	if n := strings.TrimSpace(u.Note); n != "" {
		patch["action"] = "<strong>Escalonamento:</strong> " + htmlEscapeText(n)
		patch["actionInvisibleForCaller"] = true
	}
	return patch
}

// runHTTPCalls executa os blocos http do fluxo (depois de tratar o
// chamado). Falha só vai pro log/histórico — nunca derruba o alerta.
func runHTTPCalls(fr *flowRun, ticket string) {
	for _, c := range fr.res.HTTP {
		sub := func(s string) string {
			s = strings.ReplaceAll(s, ticketNumberPlaceholder, ticket)
			return strings.ReplaceAll(s, url.QueryEscape(ticketNumberPlaceholder), url.QueryEscape(ticket))
		}
		timeout := c.Timeout
		if timeout <= 0 {
			timeout = 10
		}
		target := sub(c.URL)
		var body io.Reader
		if c.Body != "" {
			body = strings.NewReader(sub(c.Body))
		}
		req, err := http.NewRequest(c.Method, target, body)
		if err != nil {
			fr.note("http %s: %v", c.Node, err)
			log.Printf("[flow] http %s ERRO: %v\n", c.Node, err)
			continue
		}
		hasCT := false
		for _, h := range c.Headers {
			if i := strings.Index(h, ":"); i > 0 {
				k, v := strings.TrimSpace(h[:i]), strings.TrimSpace(sub(h[i+1:]))
				req.Header.Set(k, v)
				if strings.EqualFold(k, "Content-Type") {
					hasCT = true
				}
			}
		}
		if body != nil && !hasCT {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := (&http.Client{Timeout: time.Duration(timeout) * time.Second}).Do(req)
		if err != nil {
			fr.note("http %s %s %s: %v", c.Node, c.Method, target, err)
			log.Printf("[flow] http %s %s %s ERRO: %v\n", c.Node, c.Method, target, err)
			continue
		}
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		resp.Body.Close()
		fr.note("http %s %s %s → HTTP %d", c.Node, c.Method, target, resp.StatusCode)
		log.Printf("[flow] http %s %s %s → HTTP %d %s\n", c.Node, c.Method, target, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
}

// record grava a execução no histórico (fim do alerta).
func (fr *flowRun) record(cfg config.RuntimeConfig, ticket string) {
	if ticket != "" {
		fr.entry.Ticket = ticket
	}
	if err := flowlog.Append(cfg.FlowsHistoryFile, fr.entry); err != nil {
		log.Printf("[flow] WARN: falha ao gravar histórico (%s): %v\n", cfg.FlowsHistoryFile, err)
	}
}
