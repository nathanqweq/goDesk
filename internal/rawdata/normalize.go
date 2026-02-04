package rawdata

import "strings"

func Normalize(p *Payload) {
	p.Status = clean(p.Status)
	p.Host = clean(p.Host)
	p.Trigger = clean(p.Trigger)
	p.ValueItem = clean(p.ValueItem)
	p.Severity = clean(p.Severity)
	p.Date = clean(p.Date)
	p.Hour = clean(p.Hour)
	p.TriggerID = clean(p.TriggerID)
	p.EventID = clean(p.EventID)
	p.Contract = clean(p.Contract)
	p.OperGroup = clean(p.OperGroup)
	p.Operator = clean(p.Operator)
	p.MainCaller = clean(p.MainCaller)
	p.SecundaryCaller = clean(p.SecundaryCaller)
	p.Cliente = clean(p.Cliente)
	p.EventValue = clean(p.EventValue)

	// normalização defensiva
	if strings.EqualFold(p.SecundaryCaller, "null") {
		p.SecundaryCaller = ""
	}
}

func clean(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, `"`)
	s = strings.TrimSuffix(s, `"`)
	return strings.TrimSpace(s)
}
