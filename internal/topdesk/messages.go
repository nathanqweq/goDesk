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
	return fmt.Sprintf(`<html>
<META HTTP-EQUIV="Content-Type" CONTENT="text/html; charset=utf-8">
   <head>
      <title>Zabbix - %s</title>
      <style>
         <!--
            /* Font Definitions */
            @font-face
            	{font-family:"Cambria Math";
            	panose-1:2 4 5 3 5 4 6 3 2 4;}
            @font-face
            	{font-family:Calibri;
            	panose-1:2 15 5 2 2 2 4 3 2 4;}
            @font-face
            	{font-family:Verdana;
            	panose-1:2 11 6 4 3 5 4 4 2 4;}
            /* Style Definitions */
            p.MsoNormal, li.MsoNormal, div.MsoNormal
            	{margin:0cm;
            	margin-bottom:.0001pt;
            	font-size:11.0pt;
            	font-family:"Calibri",sans-serif;}
            a:link, span.MsoHyperlink
            	{mso-style-priority:99;
            	color:#0563C1;
            	text-decoration:underline;}
            a:visited, span.MsoHyperlinkFollowed
            	{mso-style-priority:99;
            	color:#954F72;
            	text-decoration:underline;}
            p.msonormal0, li.msonormal0, div.msonormal0
            	{mso-style-name:msonormal;
            	mso-margin-top-alt:auto;
            	margin-right:0cm;
            	mso-margin-bottom-alt:auto;
            	margin-left:0cm;
            	font-size:11.0pt;
            	font-family:"Calibri",sans-serif;}
            span.EstiloDeEmail18
            	{mso-style-type:personal-reply;
            	font-family:"Calibri",sans-serif;
            	color:#002060;}
            .MsoChpDefault
            	{mso-style-type:export-only;
            	font-size:10.0pt;}
            @page WordSection1
            	{size:612.0pt 792.0pt;
            	margin:70.85pt 3.0cm 70.85pt 3.0cm;}
            div.WordSection1
            	{page:WordSection1;}
            -->
      </style>
   </head>
   <body lang=PT-BR link="#0563C1" vlink="#954F72">
      <table class=MsoNormalTable border=0 cellspacing=0 cellpadding=0 width="100%%" style='width:100.0%%'>
         <tr>
            <td style='padding:0cm 0cm 0cm 0cm'>
               <div align=center>
                  <table class=MsoNormalTable border=0 cellspacing=0 cellpadding=0 width=522 style='width:391.5pt;border-collapse:collapse'>
                     <tr>
                        <td style='padding:.75pt .75pt .75pt .75pt'>
                           <div align=center>
                              <table class=MsoNormalTable border=0 cellspacing=0 cellpadding=0 width=520 style='width:390.0pt;border-collapse:collapse'>
                                 <tr>
                                    <td style='background:#E1E4E8;padding:7.5pt 0cm 3.75pt 0cm'>
                                       <div align=center>
                                          <table class=MsoNormalTable border=0 cellpadding=0 width=490 style='width:367.5pt;background:#3a6cbf'>
                                             <tr>
                                                <td style='padding:2.25pt 2.25pt 2.25pt 93.75pt'>
                                                   <table class=MsoNormalTable border=0 cellspacing=0 cellpadding=0 style='border-collapse:collapse'>
                                                      <tr>
                                                         <td style='background:#E12234;padding:0cm 9.75pt 0cm 9.75pt'>
                                                            <p class=MsoNormal align=center style='text-align:center'>
                                                               <span style='font-size:36.0pt;font-family:"Verdana",sans-serif;color:white'>Z</span>
                                                               <o:p></o:p>
                                                            </p>
                                                         </td>
                                                         <td style='background:#3a6cbf;padding:0cm 0cm 0cm 7.5pt'>
                                                            <p class=MsoNormal>
                                                               <span style='font-size:24.0pt;font-family:"Verdana",sans-serif;color:white'>Teltec Solutions<br></span>
                                                               <span style='font-size:13.5pt;font-family:"Verdana",sans-serif;color:white'>Zabbix %s</span>
                                                               <o:p></o:p>
                                                            </p>
                                                         </td>
                                                      </tr>
                                                   </table>
                                                </td>
                                             </tr>
                                          </table>
                                       </div>
                                    </td>
                                 </tr>
                                 <tr>
                                    <td style='background:#E1E4E8;padding:3.75pt 10.5pt 9.0pt 10.5pt'>
                                       <table class=MsoNormalTable border=0 cellspacing=0 cellpadding=0 width="100%%" style='width:100.0%%'>
                                          <tr>
                                             <td style='padding:.75pt .75pt .75pt .75pt'>
                                                <table class=MsoNormalTable border=0 cellspacing=0 cellpadding=0 width="100%%" style='width:100.0%%'>
                                                   <tr>
                                                      <td style='background:white;padding:3.75pt 3.75pt 3.75pt 11.25pt'>
                                                         <table class=MsoNormalTable border=0 cellspacing=0 cellpadding=0 width="100%%" style='width:100.0%%'>
                                                            <tr>
                                                               <td style='padding:2.25pt 2.25pt 2.25pt 2.25pt'>
                                                                  <p class=MsoNormal>
                                                                     <b>
                                                                        Status:
                                                                        <o:p></o:p>
                                                                     </b>
                                                                  </p>
                                                               </td>
                                                               <td style='padding:2.25pt 2.25pt 2.25pt 2.25pt'>
                                                                  <p class=MsoNormal>
                                                                     %s
                                                                     <o:p></o:p>
                                                                  </p>
                                                               </td>
                                                            </tr>
                                                            <tr>
                                                               <td style='padding:2.25pt 2.25pt 2.25pt 2.25pt'>
                                                                  <p class=MsoNormal>
                                                                     <b>
                                                                        Host:
                                                                        <o:p></o:p>
                                                                     </b>
                                                                  </p>
                                                               </td>
                                                               <td width="65%%" style='width:65.0%%;padding:2.25pt 2.25pt 2.25pt 2.25pt'>
                                                                  <p class=MsoNormal>
                                                                     %s
                                                                     <o:p></o:p>
                                                                  </p>
                                                               </td>
                                                            </tr>
                                                            <tr>
                                                               <td style='padding:2.25pt 2.25pt 2.25pt 2.25pt'>
                                                                  <p class=MsoNormal>
                                                                     <b>
                                                                        Trigger:
                                                                        <o:p></o:p>
                                                                     </b>
                                                                  </p>
                                                               </td>
                                                               <td width="65%%" style='width:65.0%%;padding:2.25pt 2.25pt 2.25pt 2.25pt'>
                                                                  <p class=MsoNormal>
                                                                     %s
                                                                     <o:p></o:p>
                                                                  </p>
                                                               </td>
                                                            </tr>
                                                            <tr>
                                                               <td style='padding:2.25pt 2.25pt 2.25pt 2.25pt'>
                                                                  <p class=MsoNormal>
                                                                     <b>
                                                                        Valor do Evento:
                                                                        <o:p></o:p>
                                                                     </b>
                                                                  </p>
                                                               </td>
                                                               <td style='padding:2.25pt 2.25pt 2.25pt 2.25pt'>
                                                                  <p class=MsoNormal>
                                                                     %s
                                                                     <o:p></o:p>
                                                                  </p>
                                                               </td>
                                                            </tr>
                                                            <tr>
                                                               <td style='padding:2.25pt 2.25pt 2.25pt 2.25pt'>
                                                                  <p class=MsoNormal>
                                                                     <b>
                                                                        Severidade:
                                                                        <o:p></o:p>
                                                                     </b>
                                                                  </p>
                                                               </td>
                                                               <td style='padding:2.25pt 2.25pt 2.25pt 2.25pt'>
                                                                  <p class=MsoNormal>
                                                                     %s
                                                                     <o:p></o:p>
                                                                  </p>
                                                               </td>
                                                            </tr>
                                                            <tr>
                                                               <td style='padding:2.25pt 2.25pt 2.25pt 2.25pt'>
                                                                  <p class=MsoNormal>
                                                                     <b>
                                                                        Data do Evento:
                                                                        <o:p></o:p>
                                                                     </b>
                                                                  </p>
                                                               </td>
                                                               <td style='padding:2.25pt 2.25pt 2.25pt 2.25pt'>
                                                                  <p class=MsoNormal>
                                                                     %s
                                                                     <o:p></o:p>
                                                                  </p>
                                                               </td>
                                                            </tr>
                                                            <tr>
                                                               <td style='padding:2.25pt 2.25pt 2.25pt 2.25pt'>
                                                                  <p class=MsoNormal>
                                                                     <b>
                                                                        Hora do Evento:
                                                                        <o:p></o:p>
                                                                     </b>
                                                                  </p>
                                                               </td>
                                                               <td style='padding:2.25pt 2.25pt 2.25pt 2.25pt'>
                                                                  <p class=MsoNormal>
                                                                     %s
                                                                     <o:p></o:p>
                                                                  </p>
                                                               </td>
                                                            </tr>
                                                            <tr>
                                                               <td style='padding:2.25pt 2.25pt 2.25pt 2.25pt'>
                                                                  <p class=MsoNormal>
                                                                     <b>
                                                                        Identificador do Evento:
                                                                        <o:p></o:p>
                                                                     </b>
                                                                  </p>
                                                               </td>
                                                               <td style='padding:2.25pt 2.25pt 2.25pt 2.25pt'>
                                                                  <p class=MsoNormal>
                                                                     %s
                                                                     <o:p></o:p>
                                                                  </p>
                                                               </td>
                                                            </tr>
                                                         </table>
                                                      </td>
                                                   </tr>
                                                </table>
                                             </td>
                                          </tr>
                                       </table>
                                    </td>
                                 </tr>
                                 <tr>
                                    <td style='padding:0cm 0cm 0cm 0cm'>
                                       <table class=MsoNormalTable border=0 cellspacing=0 cellpadding=0 width="100%%" style='width:100.0%%;background:#E1E4E8'>
                                          <tr>
                                             <td style='padding:0cm 10.5pt 7.5pt 10.5pt'>
                                                <table class=MsoNormalTable border=0 cellspacing=0 cellpadding=0 width="100%%" style='width:100.0%%'>
                                                   <tr>
                                                      <td style='background:#646D7E;padding:7.5pt 0cm 7.5pt 0cm'>
                                                         <p class=MsoNormal align=center style='text-align:center'>
                                                            <span style='font-size:10.0pt;font-family:"Verdana",sans-serif;color:white'>- Service Desk -</span>
                                                            <span style='color:white'>
                                                               <o:p></o:p>
                                                            </span>
                                                         </p>
                                                      </td>
                                                   </tr>
                                                </table>
                                             </td>
                                          </tr>
                                       </table>
                                    </td>
                                 </tr>
                              </table>
                           </div>
                        </td>
                     </tr>
                  </table>
               </div>
            </td>
         </tr>
      </table>
      <p class=MsoNormal>
         <o:p>&nbsp;</o:p>
      </p>
      </div>
   </body>
</html>`,
		htmlEscape(empty(p.Cliente, "-")),
		htmlEscape(empty(contractResolved, "-")),
		htmlEscape(empty(p.Status, "-")),
		htmlEscape(empty(p.Host, "-")),
		htmlEscape(empty(p.Trigger, "-")),
		htmlEscape(empty(prefer(p.EventValue, p.ValueItem), "-")),
		htmlEscape(empty(p.Severity, "-")),
		htmlEscape(empty(FormatDateBR(p.Date), p.Date)),
		htmlEscape(empty(p.Hour, "-")),
		htmlEscape(empty(p.EventID, "-")),
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
