package topdesk

import (
	"strings"
	"testing"

	"godesk/internal/rawdata"
)

func TestCreateHTMLIncludesEquipeOnlyWhenSet(t *testing.T) {
	p := rawdata.Payload{Host: "SW-01", Trigger: "ICMP down", Hour: "14:02:10", EventID: "999", Equipe: "NOC <N2>"}

	got := CreateHTML(p, "CONTRATO")
	want := "<strong>Hora:</strong><br>14:02:10<br><strong>Equipe:</strong><br>NOC &lt;N2&gt;<br><strong>Event ID:</strong><br>999<br>"
	if !strings.Contains(got, want) {
		t.Fatalf("esperava a linha Equipe entre Hora e Event ID, veio:\n%s", got)
	}

	p.Equipe = ""
	got = CreateHTML(p, "CONTRATO")
	if strings.Contains(got, "Equipe") {
		t.Fatalf("sem equipe a linha não deveria aparecer, veio:\n%s", got)
	}
	if !strings.Contains(got, "<strong>Hora:</strong><br>14:02:10<br><strong>Event ID:</strong>") {
		t.Fatalf("sem equipe o resto do texto deveria ficar igual, veio:\n%s", got)
	}
}
