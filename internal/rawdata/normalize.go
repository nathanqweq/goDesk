package rawdata

import "strings"

func Normalize(p *Payload) {
	// evento
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

	// novo
	p.RuleName = clean(p.RuleName)
	p.Cliente = clean(p.Cliente)

	// topdesk
	p.Contract = clean(p.Contract)
	p.OperGroup = clean(p.OperGroup)
	p.Operator = clean(p.Operator)
	p.MainCaller = clean(p.MainCaller)
	p.SecundaryCaller = clean(p.SecundaryCaller)

	// dinâmicos
	p.Sla = clean(p.Sla)
	p.Category = clean(p.Category)
	p.SubCategory = clean(p.SubCategory)
	p.CallType = clean(p.CallType)
	p.Urgency = clean(p.Urgency)
	p.Impact = clean(p.Impact)

	// futuro
	p.EntryType = clean(p.EntryType)
	p.ProcessingStatus = clean(p.ProcessingStatus)

	// normalização defensiva
	if strings.EqualFold(p.SecundaryCaller, "null") {
		p.SecundaryCaller = ""
	}
	if strings.EqualFold(p.MainCaller, "null") {
		p.MainCaller = ""
	}
	if strings.EqualFold(p.Urgency, "null") {
		p.Urgency = ""
	}
	if strings.EqualFold(p.Impact, "null") {
		p.Impact = ""
	}
}

func clean(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, `"`)
	s = strings.TrimSuffix(s, `"`)
	return strings.TrimSpace(s)
}
