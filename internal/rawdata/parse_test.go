package rawdata

import (
	"strings"
	"testing"
)

func TestParseInvalidJSONWithoutBraceIncludesMediaTypeHint(t *testing.T) {
	_, err := Parse("sd.monitoramento")
	if err == nil {
		t.Fatal("esperava erro para RAWDATA que não é JSON")
	}

	got := err.Error()
	want := "Administration → Alerts → Media types → Parameters"
	if !strings.Contains(got, want) {
		t.Fatalf("esperava dica de Media Type na mensagem, veio: %q", got)
	}
}

func TestParseInvalidJSONWithBraceOmitsMediaTypeHint(t *testing.T) {
	_, err := Parse(`{"rule_name": invalido}`)
	if err == nil {
		t.Fatal("esperava erro para JSON malformado")
	}

	got := err.Error()
	if strings.Contains(got, "Media types") {
		t.Fatalf("não esperava dica de Media Type quando RAWDATA já começa com '{', veio: %q", got)
	}
}

func TestParseEquipeOptional(t *testing.T) {
	base := `{"rule_name":"R","trigger":"T","event_id":"1","event_value":"1"`
	cases := map[string]string{
		`,"equipe":"NOC N2"}`:              "NOC N2",
		`,"Equipe":"  NOC N2 "}`:           "NOC N2", // chave com maiúscula também casa
		`,"equipe":"*UNKNOWN*"}`:           "",       // tag não existe no evento
		`,"equipe":"{EVENT.TAGS.Equipe}"}`: "",       // macro não resolvida
		`,"equipe":""}`:                    "",
		`}`:                                "", // Media Type antigo, sem o campo
	}
	for tail, want := range cases {
		p, err := Parse(base + tail)
		if err != nil {
			t.Fatalf("%s: erro inesperado: %v", tail, err)
		}
		if p.Equipe != want {
			t.Fatalf("%s: esperava equipe=%q, veio %q", tail, want, p.Equipe)
		}
	}
}
