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

	webhook := strings.TrimSpace(pol.TopDesk.TeamsWebhook)
	if webhook == "" {
		log.Printf("[teams] WARN: send_teams ativo sem teams_webhook (chamado=%q rule=%q)\n", ticketID, p.RuleName)
		return
	}

	zabbixURL := strings.TrimSpace(pol.TopDesk.TeamsZabbixURL)
	if zabbixURL == "" {
		zabbixURL = cfg.ZabbixURL
	}

	msg := teamsMessage(p, ticketID, closed)
	msg.EventURL = teams.ProblemURL(zabbixURL, p.TriggerID, p.EventID)

	log.Printf("[teams] tentativa de envio: chamado=%q rule=%q kind=%s\n", ticketID, p.RuleName, rawdata.EventKind(p))
	if err := teams.Send(httpClient, webhook, msg); err != nil {
		recordMetric(cfg.MetricsFile, func(m *metrics.Snapshot) { m.TeamsSendErrors++ })
		log.Printf("[teams] FALHA no envio: chamado=%q rule=%q erro=%v\n", ticketID, p.RuleName, err)
		return
	}
	log.Printf("[teams] OK: card enviado chamado=%q rule=%q\n", ticketID, p.RuleName)
}

// teamsMessage segue os templates PROBLEM/RECOVERY do media type "MS Teams
// Workflow" do Zabbix, com o número do chamado do TopDesk a mais.
func teamsMessage(p rawdata.Payload, ticketID string, closed bool) teams.Message {
	chamado := "Chamado TopDesk: " + ticketID
	if closed {
		chamado += " (encerrado)"
	}

	if rawdata.EventKind(p) == "ProblemStart" {
		return teams.Message{
			Subject: "Problema: " + p.Trigger,
			Style:   teams.StyleProblem,
			Lines: []string{
				"Problema iniciado às " + p.Hour + " em " + p.Date,
				"Nome do problema: " + p.Trigger,
				"Host: " + p.Host,
				"Severidade: " + p.Severity,
				"Dados operacionais: " + p.ValueItem,
				"ID do problema original: " + p.EventID,
				chamado,
			},
		}
	}

	return teams.Message{
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
