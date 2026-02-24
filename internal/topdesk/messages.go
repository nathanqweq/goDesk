package topdesk

import (
	"fmt"
	"strings"
	"time"

	"godesk/internal/rawdata"
)

func FormatDateBR(date string) string {
	date = strings.TrimSpace(date)
	t, err := time.Parse("2006.01.02", date)
	if err != nil {
		return date
	}
	return t.Format("02-01-2006")
}

func CreateHTML(p rawdata.Payload, contractResolved string) string {
	return fmt.Sprintf(
		"<strong>TELTEC SOLUTIONS</strong><br><strong>Zabbix %s</strong><br>"+
			"<strong>Status:</strong><br>%s<br>"+
			"<strong>Host:</strong><br>%s<br>"+
			"<strong>Trigger:</strong><br>%s<br>"+
			"<strong>Valor do evento:</strong><br>%s<br>"+
			"<strong>Severidade:</strong><br>%s<br>"+
			"<strong>Data:</strong><br>%s<br>"+
			"<strong>Hora:</strong><br>%s<br>"+
			"<strong>Event ID:</strong><br>%s<br>"+
			"<strong>Trigger ID:</strong><br>%s<br>",
		empty(contractResolved, "-"),
		empty(p.Status, "-"),
		empty(p.Host, "-"),
		empty(p.Trigger, "-"),
		empty(prefer(p.EventValue, p.ValueItem), "-"),
		empty(p.Severity, "-"),
		empty(FormatDateBR(p.Date), p.Date),
		empty(p.Hour, "-"),
		empty(p.EventID, "-"),
		empty(p.TriggerID, "-"),
	)
}

func CloseHTML(ticketID string, p rawdata.Payload) string {
	dataHora := strings.TrimSpace(p.Date + " " + p.Hour)
	return fmt.Sprintf(
		"Olá %s, informamos que o alerta foi normalizado<br><br>"+
			"Data e Horário: %s<br>"+
			"Chamado: %s<br>"+
			"Host: %s<br>"+
			"Alerta: %s<br>"+
			"Status: %s<br>"+
			"E, dessa forma, estamos procedendo com o encerramento do chamado.<br><br>"+
			"Atenciosamente,<br>"+
			"Equipe de Suporte e Monitoramento Teltec",
		empty(p.Cliente, "cliente"),
		empty(dataHora, "-"),
		ticketID,
		empty(p.Host, "-"),
		empty(p.Trigger, "-"),
		empty(p.Status, "-"),
	)
}

func OpeningEmailHTML(ticketID string, p rawdata.Payload, contractResolved string) string {
	intro := fmt.Sprintf(
		"Ol\u00e1 prezados,<br><br>"+
			"Informamos que estamos com o seguinte alerta em nosso monitoramento que gerou o chamado %s.<br>"+
			"Estamos verificando e em breve retornaremos com mais atualiza\u00e7\u00f5es.<br><br>",
		empty(ticketID, "-"),
	)

	return intro + CreateHTML(p, contractResolved) + "<br>Atenciosamente,<br>Equipe de Suporte e Monitoramento Teltec"
}

func empty(v, def string) string {
	v = strings.TrimSpace(v)
	if v == "" || strings.EqualFold(v, "null") || strings.EqualFold(v, "UNKNOWN") {
		return def
	}
	return v
}

func prefer(a, b string) string {
	if strings.TrimSpace(a) != "" && !strings.EqualFold(strings.TrimSpace(a), "UNKNOWN") {
		return a
	}
	return b
}
