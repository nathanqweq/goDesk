package app

import "strings"

// Regra: Zabbix > Cliente(YAML) > Default(YAML)
// "UNKNOWN" (case-insensitive) conta como vazio
func pickTag(zbxVal, clientVal, defVal string) string {
	if v := normTag(zbxVal); v != "" {
		return v
	}
	if v := normTag(clientVal); v != "" {
		return v
	}
	if v := normTag(defVal); v != "" {
		return v
	}
	return ""
}

func normTag(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.EqualFold(s, "UNKNOWN") {
		return ""
	}
	return s
}
