package app

import (
	"log"
	"net/http"
	"strings"
	"time"

	"godesk/internal/config"
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

	// Policy final (default + override do cliente)
	pol := config.ResolvePolicy(pf, p.Cliente)

	// Resolve TAGs (Zabbix > Cliente(YAML) > Default(YAML))
	contract := pickTag(p.Contract, pol.Tags.Contract, pf.Default.Tags.Contract)
	log.Printf("[debug] contract resolved=%q len=%d bytes=%v\n", contract, len(contract), []byte(contract))

	operGrp := pickTag(p.OperGroup, pol.Tags.OperGroup, pf.Default.Tags.OperGroup)
	log.Printf("[debug] operGrp resolved=%q len=%d bytes=%v\n", operGrp, len(operGrp), []byte(operGrp))

	mainCaller := pickTag(p.MainCaller, pol.Tags.MainCaller, pf.Default.Tags.MainCaller)
	log.Printf("[debug] mainCaller resolved=%q len=%d bytes=%v\n", mainCaller, len(mainCaller), []byte(mainCaller))

	secCaller := pickTag(p.SecundaryCaller, pol.Tags.SecundaryCaller, pf.Default.Tags.SecundaryCaller)
	log.Printf("[debug] secCaller resolved=%q len=%d bytes=%v\n", secCaller, len(secCaller), []byte(secCaller))

	// sanity checks (pra não criar ticket quebrado)
	if strings.TrimSpace(mainCaller) == "" {
		log.Printf("[app] WARN: mainCaller ficou vazio após resolução (cliente=%q)\n", p.Cliente)
	}
	if strings.TrimSpace(operGrp) == "" {
		log.Printf("[app] WARN: oper_group ficou vazio após resolução (cliente=%q)\n", p.Cliente)
	}
	if strings.TrimSpace(contract) == "" {
		log.Printf("[app] WARN: contract ficou vazio após resolução (cliente=%q)\n", p.Cliente)
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
		BaseURL: cfg.ZabbixURL,
		Token:   cfg.ZabbixKey,
		HTTP:    httpClient,
		Timeout: timeout,
	}

	eventKind := rawdata.EventKind(p)
	log.Printf("[app] kind=%s cliente=%q autoclose=%v urgency=%q impact=%q ticket=%q\n",
		eventKind, p.Cliente, pol.AutoClose, pol.Urgency, pol.Impact, cfg.TicketName)

	exists, ticketID, status, err := td.TicketExists(cfg.TicketName)
	if err != nil {
		return err
	}

	switch {
	case !exists && eventKind == "ProblemStart":
		msgHTML := topdesk.CreateHTML(p, contract)
		payload := buildCreatePayload(cfg.TicketName, msgHTML, p, pol, contract, operGrp, mainCaller, secCaller)

		created, err := td.CreateTicket(payload)
		if err != nil {
			return err
		}
		_ = zx.Acknowledge(p.EventID, "Chamado criado: "+created)

	case exists && eventKind == "ProblemStart":
		action := "Recebemos novamente o alerta " + p.Trigger + " em nosso Zabbix."
		_ = td.PatchTicket(ticketID, buildUpdatePayload(action, status))
		_ = zx.Acknowledge(p.EventID, "Chamado já existe: "+ticketID)

	case exists && eventKind == "ProblemRecovery":
		if strings.EqualFold(status, "Fechado") {
			_ = zx.Acknowledge(p.EventID, "Chamado já estava fechado: "+ticketID)
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
			_ = zx.Acknowledge(p.EventID, "Chamado encerrado: "+ticketID)
		} else {
			action := "Recebemos a normalização do alerta " + p.Trigger + " em nosso Zabbix."
			_ = td.PatchTicket(ticketID, buildUpdatePayload(action, status))
			_ = zx.Acknowledge(p.EventID, "Normalização recebida (sem autoclose): "+ticketID)
		}
	}

	return nil
}

func buildCreatePayload(ticketName, msgHTML string, p rawdata.Payload, pol config.Policy,
	contract, operGrp, mainCaller, secCaller string,
) map[string]any {
	brief := ticketName
	if len(brief) > 79 {
		brief = brief[:79]
	}

	return map[string]any{
		"callerLookup":     map[string]any{"email": mainCaller},
		"briefDescription": brief,
		"request":          msgHTML,
		"entryType":        map[string]any{"name": "Web"},
		"callType":         map[string]any{"name": "Resolução de incidente"},
		"category":         map[string]any{"name": "7 - Monitoramento"},
		"subcategory":      map[string]any{"name": "Monitoramento " + contract},
		"impact":           map[string]any{"name": pol.Impact},
		"urgency":          map[string]any{"name": pol.Urgency},
		"operatorGroup":    map[string]any{"id": operGrp},
		"operator":         map[string]any{"id": "71853e6f-c50a-4600-82cd-24752449d803"},
		"processingStatus": map[string]any{"name": "Registrado"},
		"optionalFields2":  map[string]any{"memo2": secCaller},
	}
}

func buildUpdatePayload(action, currentStatus string) map[string]any {
	payload := map[string]any{"action": action}
	if !strings.EqualFold(currentStatus, "Registrado") {
		payload["processingStatus"] = map[string]any{"id": ClosedStatusID}
	}
	return payload
}
