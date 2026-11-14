package app

import (
	"log"
	"net/http"
	"strings"

	"godesk/internal/config"
	"godesk/internal/metrics"
	"godesk/internal/rawdata"
	"godesk/internal/teams"
)

// notifyTeams posta o card do evento no Workflow do Teams quando a policy
// tem send_teams ligado. Best-effort: falha só loga e conta métrica, nunca
// altera o resultado do processamento do alerta (o chamado já foi
// criado/atualizado a essa altura).
func notifyTeams(cfg config.RuntimeConfig, httpClient *http.Client, pol config.Policy, p rawdata.Payload, ticketID string, closed bool) {
	if !pol.TopDesk.SendTeams {
		return
	}
	sendTeamsCard(cfg, httpClient, pol, p, ticketID, teamsMessage(p, ticketID, closed))
}

// sendTeamsCard manda o card pra cada webhook da policy (teams_webhook
// aceita vários, separados por vírgula, ponto e vírgula ou quebra de linha
// — ex: o fluxo manda o crítico pro canal do plantão e pro geral).
func sendTeamsCard(cfg config.RuntimeConfig, httpClient *http.Client, pol config.Policy, p rawdata.Payload, ticketID string, msg teams.Message) {
	hooks := teamsWebhooks(pol.TopDesk.TeamsWebhook)
	if len(hooks) == 0 {
		log.Printf("[teams] WARN: send_teams ativo sem teams_webhook (chamado=%q rule=%q)\n", ticketID, p.RuleName)
		return
	}

	zabbixURL := strings.TrimSpace(pol.TopDesk.TeamsZabbixURL)
	if zabbixURL == "" {
		zabbixURL = cfg.ZabbixURL
	}
	msg.EventURL = teams.ProblemURL(zabbixURL, p.TriggerID, p.EventID)

	for _, webhook := range hooks {
		log.Printf("[teams] tentativa de envio: chamado=%q rule=%q kind=%s\n", ticketID, p.RuleName, rawdata.EventKind(p))
		if err := teams.Send(httpClient, webhook, msg); err != nil {
			recordMetric(cfg.MetricsFile, func(m *metrics.Snapshot) { m.TeamsSendErrors++ })
			log.Printf("[teams] FALHA no envio: chamado=%q rule=%q erro=%v\n", ticketID, p.RuleName, err)
			continue
		}
		log.Printf("[teams] OK: card enviado chamado=%q rule=%q\n", ticketID, p.RuleName)
	}
}

func teamsWebhooks(s string) []string {
	var out []string
	for _, h := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == '\n' || r == '\r' }) {
		if h = strings.TrimSpace(h); h != "" {
			out = append(out, h)
		}
	}
	return out
}

// teamsMessage segue os templates PROBLEM/RECOVERY do media type "MS Teams
// Workflow" do Zabbix, com o número do chamado do TopDesk a mais. O fluxo
// pode trocar o título (teams_title) e acrescentar texto (teams_message).
func teamsMessage(p rawdata.Payload, ticketID string, closed bool) teams.Message {
	chamado := "Chamado TopDesk: " + ticketID
	if closed {
		chamado += " (encerrado)"
	}

	var m teams.Message
	if rawdata.EventKind(p) == "ProblemStart" {
		lines := []string{
			"Problema iniciado às " + p.Hour + " em " + p.Date,
			"Nome do problema: " + p.Trigger,
			"Host: " + p.Host,
			"Severidade: " + p.Severity,
			"Dados operacionais: " + p.ValueItem,
			"ID do problema original: " + p.EventID,
			chamado,
		}
		// instrução do fluxo (ex: "Ligar para o cliente") em destaque, logo
		// abaixo do título
		if v := strings.TrimSpace(p.Instrucao); v != "" {
			lines = append([]string{"**Instrução: " + v + "**"}, lines...)
		}
		m = teams.Message{Subject: "Problema: " + p.Trigger, Style: teams.StyleProblem, Lines: lines}
	} else {
		m = teams.Message{
			Subject: "Resolvido: " + p.Trigger,
			Style:   teams.StyleResolve,
			Lines: []string{
				"Problema resolvido (iniciado às " + p.Hour + " em " + p.Date + ")",
				"Nome do problema: " + p.Trigger,
				"Host: " + p.Host,
				"Severidade: " + p.Severity,
				"ID do problema original: " + p.EventID,
				chamado,
			},
		}
	}
	return customizeTeams(m, p)
}

// customizeTeams aplica teams_title / teams_message do fluxo: o título
// substitui o assunto; cada linha da mensagem entra logo abaixo do título.
func customizeTeams(m teams.Message, p rawdata.Payload) teams.Message {
	if v := strings.TrimSpace(p.TeamsTitle); v != "" {
		m.Subject = v
	}
	if v := strings.TrimSpace(p.TeamsMessage); v != "" {
		var extra []string
		for _, l := range strings.Split(v, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				extra = append(extra, l)
			}
		}
		m.Lines = append(extra, m.Lines...)
	}
	return m
}

// escalationTeamsMessage: card do bloco "atualizar chamado" com Teams.
func escalationTeamsMessage(p rawdata.Payload, ticketID, note string) teams.Message {
	lines := []string{}
	if n := strings.TrimSpace(note); n != "" {
		lines = append(lines, "**"+n+"**")
	}
	lines = append(lines,
		"Problema iniciado às "+p.Hour+" em "+p.Date,
		"Host: "+p.Host,
		"Severidade: "+p.Severity,
		"ID do problema original: "+p.EventID,
		"Chamado TopDesk: "+ticketID,
	)
	return customizeTeams(teams.Message{Subject: "Escalonamento: " + p.Trigger, Style: teams.StyleProblem, Lines: lines}, p)
}
