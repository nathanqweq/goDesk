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

	// Resolve campos TopDesk (Zabbix > Cliente(YAML) > Default(YAML))
	contract := pickTag(p.Contract, pol.TopDesk.Contract, pf.Default.TopDesk.Contract)

	operator := pickTag(p.Operator, pol.TopDesk.Operator, pf.Default.TopDesk.Operator)
	operGrp := pickTag(p.OperGroup, pol.TopDesk.OperGroup, pf.Default.TopDesk.OperGroup)

	mainCaller := pickTag(p.MainCaller, pol.TopDesk.MainCaller, pf.Default.TopDesk.MainCaller)
	secCaller := pickTag(p.SecundaryCaller, pol.TopDesk.SecundaryCaller, pf.Default.TopDesk.SecundaryCaller)

	slaID := pickTag(p.Sla, pol.TopDesk.Sla, pf.Default.TopDesk.Sla)
	category := pickTag(p.Category, pol.TopDesk.Category, pf.Default.TopDesk.Category)
	subCategory := pickTag(p.SubCategory, pol.TopDesk.SubCategory, pf.Default.TopDesk.SubCategory)
	callType := pickTag(p.CallType, pol.TopDesk.CallType, pf.Default.TopDesk.CallType)

	// sanity checks (pra não criar ticket quebrado)
	if strings.TrimSpace(mainCaller) == "" {
		log.Printf("[app] WARN: mainCaller ficou vazio após resolução (cliente=%q)\n", p.Cliente)
	}
	if strings.TrimSpace(operGrp) == "" {
		log.Printf("[app] WARN: oper_group ficou vazio após resolução (cliente=%q)\n", p.Cliente)
	}
	if strings.TrimSpace(operator) == "" {
		log.Printf("[app] WARN: operator ficou vazio após resolução (cliente=%q)\n", p.Cliente)
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

		payload := buildCreatePayload(
			cfg.TicketName,
			msgHTML,
			p,
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
		)

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

func buildCreatePayload(
	ticketName, msgHTML string,
	p rawdata.Payload,
	pol config.Policy,
	contract, operator, operGrp, mainCaller, secCaller string,
	slaID, category, subCategory, callType string,
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

	// overrides via YAML/Zabbix (se vierem)
	if strings.TrimSpace(callType) != "" {
		callTypeName = callType
	}
	if strings.TrimSpace(category) != "" {
		categoryName = category
	}
	if strings.TrimSpace(subCategory) != "" {
		subCategoryName = subCategory
	}

	payload := map[string]any{
		"callerLookup":     map[string]any{"email": mainCaller},
		"briefDescription": brief,
		"request":          msgHTML,
		"entryType":        map[string]any{"name": entryTypeName},
		"callType":         map[string]any{"name": callTypeName},
		"category":         map[string]any{"name": categoryName},
		"subcategory":      map[string]any{"name": subCategoryName},
		"impact":           map[string]any{"name": pol.Impact},
		"urgency":          map[string]any{"name": pol.Urgency},
		"operatorGroup":    map[string]any{"id": operGrp},
		"operator":         map[string]any{"id": operator},
		"processingStatus": map[string]any{"name": processingStatusName},
		"optionalFields2":  map[string]any{"memo2": secCaller},
	}

	// SLA é objeto por ID (somente se tiver valor)
	if strings.TrimSpace(slaID) != "" {
		payload["sla"] = map[string]any{"id": slaID}
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
