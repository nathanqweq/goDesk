package topdesk

import (
	"strings"
	"testing"

	"godesk/internal/rawdata"
)

func TestCreateHTMLIncludesEquipeOnlyWhenSet(t *testing.T) {
	p := rawdata.Payload{Host: "SW-01", Trigger: "ICMP down", Equipe: "NOC <N2>"}

	got := CreateHTML(p, "CONTRATO")
	want := "<strong>Host:</strong><br>SW-01<br><strong>Equipe:</strong><br>NOC &lt;N2&gt;<br><strong>Trigger:</strong>"
	if !strings.Contains(got, want) {
		t.Fatalf("esperava a linha Equipe logo depois do Host, veio:\n%s", got)
	}

	p.Equipe = ""
	got = CreateHTML(p, "CONTRATO")
	if strings.Contains(got, "Equipe") {
		t.Fatalf("sem equipe a linha não deveria aparecer, veio:\n%s", got)
	}
	if !strings.Contains(got, "<strong>Host:</strong><br>SW-01<br><strong>Trigger:</strong>") {
		t.Fatalf("sem equipe o resto do texto deveria ficar igual, veio:\n%s", got)
	}
}
