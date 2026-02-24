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
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
  <head>
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8">
    <title>Zabbix - Teltec Solutions</title>
    <style>
      body { margin: 0; padding: 16px; background: #f3f5f7; font-family: Calibri, Arial, sans-serif; color: #2b2b2b; }
      .mail-wrap { max-width: 760px; margin: 0 auto; background: #ffffff; border: 1px solid #e1e4e8; }
      .head { background: #3a6cbf; color: #fff; padding: 16px 20px; font-family: Verdana, Arial, sans-serif; }
      .head .kicker { font-size: 12px; opacity: 0.9; }
      .head .title { font-size: 22px; font-weight: 700; margin-top: 4px; }
      .content { padding: 18px 20px; font-size: 14px; line-height: 1.45; }
      .ticket { margin: 10px 0 18px 0; padding: 10px 12px; background: #f8fafc; border-left: 4px solid #3a6cbf; }
      .grid { width: 100%%; border-collapse: collapse; margin-top: 10px; }
      .grid td { border: 1px solid #e6e6e6; padding: 8px 10px; vertical-align: top; }
      .grid td:first-child { width: 34%%; font-weight: 700; background: #fafafa; }
      .foot { background: #646D7E; color: #fff; text-align: center; font-size: 12px; padding: 10px; font-family: Verdana, Arial, sans-serif; }
    </style>
  </head>
  <body>
    <div class="mail-wrap">
      <div class="head">
        <div class="kicker">Zabbix</div>
        <div class="title">Teltec Solutions - %s</div>
      </div>
      <div class="content">
        <p>Ola prezados,</p>
        <p>Informamos que estamos com o seguinte alerta em nosso monitoramento que gerou o chamado <b>%s</b>.
        Estamos verificando e em breve retornaremos com mais atualizacoes.</p>
        <div class="ticket">Assunto do chamado: <b>%s</b></div>
        <table class="grid">
          <tr><td>Status</td><td>%s</td></tr>
          <tr><td>Host</td><td>%s</td></tr>
          <tr><td>Trigger</td><td>%s</td></tr>
          <tr><td>Valor do evento</td><td>%s</td></tr>
          <tr><td>Severidade</td><td>%s</td></tr>
          <tr><td>Data do evento</td><td>%s</td></tr>
          <tr><td>Hora do evento</td><td>%s</td></tr>
          <tr><td>Event ID</td><td>%s</td></tr>
          <tr><td>Trigger ID</td><td>%s</td></tr>
        </table>
        <p style="margin-top:14px;">Atenciosamente,<br>Equipe de Suporte e Monitoramento Teltec</p>
      </div>
      <div class="foot">- Service Desk -</div>
    </div>
  </body>
</html>`,
		htmlEscape(empty(contractResolved, "-")),
		htmlEscape(empty(ticketID, "-")),
		htmlEscape(empty(ticketID, "-")),
		htmlEscape(empty(p.Status, "-")),
		htmlEscape(empty(p.Host, "-")),
		htmlEscape(empty(p.Trigger, "-")),
		htmlEscape(empty(prefer(p.EventValue, p.ValueItem), "-")),
		htmlEscape(empty(p.Severity, "-")),
		htmlEscape(empty(FormatDateBR(p.Date), p.Date)),
		htmlEscape(empty(p.Hour, "-")),
		htmlEscape(empty(p.EventID, "-")),
		htmlEscape(empty(p.TriggerID, "-")),
	)
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

func htmlEscape(s string) string {
	repl := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return repl.Replace(s)
}
