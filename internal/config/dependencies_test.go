package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDependenciesDefaultsModeToAny(t *testing.T) {
	df, err := ParseDependencies([]byte(`
dependencies:
  - host: SRV-01
    parents:
      - host: SW-01
`))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if df.Dependencies[0].Mode != DependencyModeAny {
		t.Fatalf("esperava mode=any, veio %q", df.Dependencies[0].Mode)
	}
}

func TestParseDependenciesRejectsInvalid(t *testing.T) {
	cases := map[string]string{
		"host vazio": `
dependencies:
  - parents: [{host: SW-01}]`,
		"sem pais": `
dependencies:
  - host: SRV-01`,
		"mode inválido": `
dependencies:
  - host: SRV-01
    mode: some
    parents: [{host: SW-01}]`,
		"depende de si": `
dependencies:
  - host: SRV-01
    parents: [{host: srv-01}]`,
		"duplicado": `
dependencies:
  - host: SRV-01
    parents: [{host: SW-01}]
  - host: srv-01
    parents: [{host: SW-02}]`,
		"ciclo": `
dependencies:
  - host: A
    parents: [{host: B}]
  - host: B
    parents: [{host: C}]
  - host: C
    parents: [{host: A}]`,
	}

	for name, y := range cases {
		if _, err := ParseDependencies([]byte(y)); err == nil {
			t.Errorf("%s: esperava erro", name)
		}
	}
}

func TestParseDependenciesCycleMessageShowsPath(t *testing.T) {
	_, err := ParseDependencies([]byte(`
dependencies:
  - host: A
    parents: [{host: B}]
  - host: B
    parents: [{host: A}]
`))
	if err == nil || !strings.Contains(err.Error(), "→") {
		t.Fatalf("esperava erro com o caminho do ciclo, veio %v", err)
	}
}

func TestFindPrefersHostIDThenName(t *testing.T) {
	df := DependenciesFile{Dependencies: []Dependency{
		{Host: "SRV-NOME-ANTIGO", HostID: "100"},
		{Host: "SRV-02", HostID: "999"},
	}}

	if d, ok := df.Find("100", "SRV-RENOMEADO"); !ok || d.HostID != "100" {
		t.Fatalf("esperava achar pelo hostid, veio %+v ok=%v", d, ok)
	}
	// hostid de outro Zabbix (arquivo importado): cai pro nome
	if d, ok := df.Find("555", "srv-02"); !ok || d.Host != "SRV-02" {
		t.Fatalf("esperava achar pelo nome, veio %+v ok=%v", d, ok)
	}
	if _, ok := df.Find("555", "OUTRO"); ok {
		t.Fatal("não esperava achar")
	}
}

func TestLoadDependenciesMissingFileIsEmpty(t *testing.T) {
	df, err := LoadDependencies(filepath.Join(t.TempDir(), "nao-existe.yaml"))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(df.Dependencies) != 0 {
		t.Fatal("esperava vazio")
	}
}

func TestDependencyFlagBelongsToRule(t *testing.T) {
	pf, err := ParsePolicies([]byte(`
default:
  topdesk: {}
clients:
  CLI:
    topdesk: {contract: X}
rules:
  COM-DEP:
    client: CLI
    dependency: true
  SEM-DEP:
    client: CLI
`))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !ResolvePolicy(pf, "COM-DEP").Dependency {
		t.Fatal("esperava dependency=true")
	}
	if ResolvePolicy(pf, "SEM-DEP").Dependency {
		t.Fatal("esperava dependency=false")
	}
}
