package rawdata

import "strings"

func Normalize(p *Payload) {
	// ===== EVENTO =====
	p.Status = clean(p.Status)
	p.Host = clean(p.Host)
	p.Trigger = clean(p.Trigger)
	p.ValueItem = clean(p.ValueItem)
	p.Severity = clean(p.Severity)
	p.Date = clean(p.Date)
	p.Hour = clean(p.Hour)
	p.TriggerID = clean(p.TriggerID)
	p.EventID = clean(p.EventID)
	p.EventValue = clean(p.EventValue)
	p.Cliente = clean(p.Cliente)

	// ===== TAGS / TOPDESK =====
	p.Contract = clean(p.Contract)
	p.OperGroup = clean(p.OperGroup)
	p.Operator = clean(p.Operator)
	p.MainCaller = clean(p.MainCaller)
	p.SecundaryCaller = clean(p.SecundaryCaller)

	// ===== NOVOS CAMPOS DINÂMICOS =====
	p.Sla = clean(p.Sla)
	p.Category = clean(p.Category)
	p.SubCategory = clean(p.SubCategory)
	p.CallType = clean(p.CallType)

	// futuro (se você ativar no payload)
	p.EntryType = clean(p.EntryType)
	p.ProcessingStatus = clean(p.ProcessingStatus)

	// normalização defensiva
	if strings.EqualFold(p.SecundaryCaller, "null") {
		p.SecundaryCaller = ""
	}
	if strings.EqualFold(p.MainCaller, "null") {
		p.MainCaller = ""
	}
}

func clean(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, `"`)
	s = strings.TrimSuffix(s, `"`)
	return strings.TrimSpace(s)
}
