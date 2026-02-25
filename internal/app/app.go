package app

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"godesk/internal/config"
	"godesk/internal/mailer"
	"godesk/internal/rawdata"
	"godesk/internal/topdesk"
	"godesk/internal/zabbix"
)

func Run(cfg config.RuntimeConfig) error {
	p, err := rawdata.Parse(cfg.RawData)
	if err != nil {
		return err
	}

	pf, err := config.LoadPolicies(cfg.ConfigFile)
	if err != nil {
		return err
	}

	// Policy final (default + override da RULE)
	pol := config.ResolvePolicy(pf, p.RuleName)

	// Nome bonito de cliente (display): Zabbix > YAML rule > YAML default
	displayClient := pickTag(p.Cliente, pol.Client, pf.Default.Client)
	// mantém compatibilidade com seu HTML atual (se ele usa p.Cliente)
	p.Cliente = displayClient

	// Resolve campos TopDesk (Zabbix > Rule(YAML) > Default(YAML))
	contract := pickTag(p.Contract, pol.TopDesk.Contract, pf.Default.TopDesk.Contract)

	operator := pickTag(p.Operator, pol.TopDesk.Operator, pf.Default.TopDesk.Operator)
	operGrp := pickTag(p.OperGroup, pol.TopDesk.OperGroup, pf.Default.TopDesk.OperGroup)

	mainCaller := pickTag(p.MainCaller, pol.TopDesk.MainCaller, pf.Default.TopDesk.MainCaller)
	secCaller := pickTag(p.SecundaryCaller, pol.TopDesk.SecundaryCaller, pf.Default.TopDesk.SecundaryCaller)

	slaID := pickTag(p.Sla, pol.TopDesk.Sla, pf.Default.TopDesk.Sla)
	category := pickTag(p.Category, pol.TopDesk.Category, pf.Default.TopDesk.Category)
	subCategory := pickTag(p.SubCategory, pol.TopDesk.SubCategory, pf.Default.TopDesk.SubCategory)
	callType := pickTag(p.CallType, pol.TopDesk.CallType, pf.Default.TopDesk.CallType)

	// Urgency/Impact/Priority seguem hierarquia: Zabbix > rule > default
	pol.Urgency = pickTag(p.Urgency, pol.Urgency, pf.Default.Urgency)
	pol.Impact = pickTag(p.Impact, pol.Impact, pf.Default.Impact)
	priority := pickTag(p.Priority, pol.Priority, pf.Default.Priority)

	// sanity checks (pra não criar ticket quebrado)
	if strings.TrimSpace(mainCaller) == "" {
		log.Printf("[app] WARN: mainCaller ficou vazio após resolução (rule=%q cliente=%q)\n", p.RuleName, p.Cliente)
	}
	if strings.TrimSpace(operGrp) == "" {
		log.Printf("[app] WARN: oper_group ficou vazio após resolução (rule=%q cliente=%q)\n", p.RuleName, p.Cliente)
	}
	if strings.TrimSpace(operator) == "" {
		log.Printf("[app] WARN: operator ficou vazio após resolução (rule=%q cliente=%q)\n", p.RuleName, p.Cliente)
	}
	if strings.TrimSpace(contract) == "" {
		log.Printf("[app] WARN: contract ficou vazio após resolução (rule=%q cliente=%q)\n", p.RuleName, p.Cliente)
	}

	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	httpClient := &http.Client{Timeout: timeout}

	td := topdesk.Client{
		BaseURL: cfg.Domain,
		User:    cfg.User,
		Pass:    cfg.Pass,
		HTTP:    httpClient,
	}

	zx := zabbix.Client{
		BaseURL:     cfg.ZabbixURL,
		Token:       cfg.ZabbixKey,
		HTTP:        httpClient,
		Timeout:     timeout,
		InsecureTLS: true,
	}

	eventKind := rawdata.EventKind(p)
	log.Printf("[app] kind=%s rule=%q cliente=%q autoclose=%v urgency=%q impact=%q priority=%q ticket=%q\n",
		eventKind, p.RuleName, p.Cliente, pol.AutoClose, pol.Urgency, pol.Impact, priority, cfg.TicketName)

	exists, ticketID, status, err := td.TicketExists(cfg.TicketName)
	if err != nil {
		return err
	}

	switch {
	case !exists && eventKind == "ProblemStart":
		msgHTML := topdesk.CreateHTML(p, contract)

		payload := buildCreatePayload(
			cfg.TicketName,
			msgHTML,
			pol,
			contract,
			operator,
			operGrp,
			mainCaller,
			secCaller,
			slaID,
			category,
			subCategory,
			callType,
			priority,
			p.Hour,
			p.Severity,
		)

		created, err := td.CreateTicket(payload)
		if err != nil {
			return err
		}
		if pol.TopDesk.SendMoreInfo {
			infoMsg := strings.TrimSpace(pol.TopDesk.MoreInfoText)
			if infoMsg != "" && !strings.EqualFold(infoMsg, "null") {
				if err := td.PatchTicket(created, map[string]any{
					"action":                   infoMsg,
					"actionInvisibleForCaller": true,
				}); err != nil {
					log.Printf("[topdesk] WARN: falha ao enviar send_more_info ticket=%s: %v\n", created, err)
				}
			} else {
				log.Printf("[topdesk] WARN: send_more_info ativo sem texto configurado (rule=%q)\n", p.RuleName)
			}
		}
		if pol.TopDesk.SendEmail {
			to := mailer.ParseRecipients(pol.TopDesk.EmailTo)
			cc := mailer.ParseRecipients(pol.TopDesk.EmailCc)

			if len(to) == 0 {
				log.Printf("[email] WARN: send_email ativo sem destinatario TO (rule=%q)\n", p.RuleName)
			} else {
				subject := fmt.Sprintf("%s - %s", cfg.TicketName, created)
				body := topdesk.OpeningEmailHTML(created, p, contract)
				err := mailer.SendHTML(
					mailer.Config{
						Host: cfg.SMTPHost,
						Port: cfg.SMTPPort,
						User: cfg.SMTPUser,
						Pass: cfg.SMTPPass,
						From: cfg.SMTPFrom,
					},
					to,
					cc,
					subject,
					body,
				)
				if err != nil {
					log.Printf("[email] WARN: falha ao enviar email ticket=%s: %v\n", created, err)
				}
			}
		}
		if err := zx.Acknowledge(p.EventID, "Chamado criado: "+created); err != nil {
			log.Printf("[zabbix] ACK ERROR: %v\n", err)
		}

	case exists && eventKind == "ProblemStart":
		action := "Recebemos novamente o alerta " + p.Trigger + " em nosso Zabbix."
		_ = td.PatchTicket(ticketID, buildUpdatePayload(action, status))
		if err := zx.Acknowledge(p.EventID, "Chamado já existe: "+ticketID); err != nil {
			log.Printf("[zabbix] ACK ERROR: %v\n", err)
		}

	case exists && eventKind == "ProblemRecovery":
		if strings.EqualFold(status, "Fechado") {
			if err := zx.Acknowledge(p.EventID, "Chamado já estava fechado: "+ticketID); err != nil {
				log.Printf("[zabbix] ACK ERROR: %v\n", err)
			}
			return nil
		}

		if pol.AutoClose {
			closeMsg := topdesk.CloseHTML(ticketID, p)
			_ = td.PatchTicket(ticketID, map[string]any{
				"action": closeMsg,
				"processingStatus": map[string]any{
					"name": "Fechado",
				},
			})
			if err := zx.Acknowledge(p.EventID, "Chamado encerrado: "+ticketID); err != nil {
				log.Printf("[zabbix] ACK ERROR: %v\n", err)
			}
		} else {
			action := "Recebemos a normalização do alerta " + p.Trigger + " em nosso Zabbix."
			_ = td.PatchTicket(ticketID, buildUpdatePayload(action, status))
			if err := zx.Acknowledge(p.EventID, "Normalização recebida (sem autoclose): "+ticketID); err != nil {
				log.Printf("[zabbix] ACK ERROR: %v\n", err)
			}
		}
	}

	return nil
}

func buildCreatePayload(
	ticketName, msgHTML string,
	pol config.Policy,
	contract, operator, operGrp, mainCaller, secCaller string,
	slaID, category, subCategory, callType, priority, hour, severity string,
) map[string]any {
	brief := ticketName
	if len(brief) > 79 {
		brief = brief[:79]
	}

	// defaults (mantém seu padrão atual)
	entryTypeName := "Web"
	callTypeName := "Resolução de incidente"
	categoryName := "7 - Monitoramento"
	subCategoryName := "Monitoramento " + contract
	processingStatusName := "Registrado"

	// overrides via Zabbix/YAML (se vier vazio, mantém defaults)
	if strings.TrimSpace(callType) != "" && !strings.EqualFold(strings.TrimSpace(callType), "null") {
		callTypeName = strings.TrimSpace(callType)
	}
	if strings.TrimSpace(category) != "" && !strings.EqualFold(strings.TrimSpace(category), "null") {
		categoryName = strings.TrimSpace(category)
	}
	if strings.TrimSpace(subCategory) != "" && !strings.EqualFold(strings.TrimSpace(subCategory), "null") {
		subCategoryName = strings.TrimSpace(subCategory)
	}

	payload := map[string]any{
		"callerLookup":     map[string]any{"email": mainCaller},
		"briefDescription": brief,
		"request":          msgHTML,
		"entryType":        map[string]any{"name": entryTypeName},
		"callType":         map[string]any{"name": callTypeName},
		"category":         map[string]any{"name": categoryName},
		"subcategory":      map[string]any{"name": subCategoryName},
		"operatorGroup":    map[string]any{"id": operGrp},
		"operator":         map[string]any{"id": operator},
		"processingStatus": map[string]any{"name": processingStatusName},
	}

	// Secondary caller só é enviado quando existir valor válido.
	if v := strings.TrimSpace(secCaller); v != "" && !strings.EqualFold(v, "null") {
		payload["optionalFields2"] = map[string]any{"memo2": v}
	}

	if pol.TopDesk.AdicionalCresol {
		payload["optionalFields1"] = map[string]any{
			"text1": strings.TrimSpace(hour),
			"text2": strings.TrimSpace(ticketName),
			"text3": strings.TrimSpace(severity),
		}
	}

	// ✅ Impact só vai se existir (evita 400 por name inexistente/vazio)
	if v := strings.TrimSpace(pol.Impact); v != "" && !strings.EqualFold(v, "null") {
		payload["impact"] = map[string]any{"name": v}
	}

	// ✅ Urgency só vai se existir (evita 400 por name inexistente/vazio)
	if v := strings.TrimSpace(pol.Urgency); v != "" && !strings.EqualFold(v, "null") {
		payload["urgency"] = map[string]any{"name": v}
	}

	// SLA é enviado sempre que existir (independente de autoclose)
	if v := strings.TrimSpace(slaID); v != "" && !strings.EqualFold(v, "null") {
		payload["sla"] = map[string]any{"id": v}
	}

	// Priority opcional
	if v := strings.TrimSpace(priority); v != "" && !strings.EqualFold(v, "null") {
		payload["priority"] = map[string]any{"name": v}
	}

	return payload
}

func buildUpdatePayload(action, currentStatus string) map[string]any {
	payload := map[string]any{"action": action}
	if !strings.EqualFold(currentStatus, "Registrado") {
		payload["processingStatus"] = map[string]any{"id": ClosedStatusID}
	}
	return payload
}
