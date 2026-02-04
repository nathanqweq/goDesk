package app

import "strings"

// Regra: Zabbix > Cliente(YAML) > Default(YAML)
// Considera vazio: "", "UNKNOWN", "*UNKNOWN*", "<UNKNOWN>", "(UNKNOWN)" etc (case-insensitive)
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

	// remove wrappers comuns que o Zabbix pode colocar
	// ex: "*UNKNOWN*", "<UNKNOWN>", "\"UNKNOWN\""
	s = strings.TrimSpace(strings.Trim(s, `"'*<>[](){} `))

	// remove quebras
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\t", "")
	s = strings.TrimSpace(s)

	// se ainda ficou vazio
	if s == "" {
		return ""
	}

	// unknown em qualquer variação
	if strings.EqualFold(s, "UNKNOWN") {
		return ""
	}

	return s
}
