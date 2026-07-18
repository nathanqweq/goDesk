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
