package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"godesk/internal/config"
	"godesk/internal/rawdata"
	"godesk/internal/zabbix"
)

type fakeZabbix struct {
	hosts    map[string]zabbix.Host
	problems map[string][]zabbix.Problem
	failOn   string
}

func (f fakeZabbix) HostsByName(names []string) (map[string]zabbix.Host, error) {
	out := map[string]zabbix.Host{}
	for _, n := range names {
		if h, ok := f.hosts[strings.ToLower(n)]; ok {
			out[strings.ToLower(n)] = h
		}
	}
	return out, nil
}

func (f fakeZabbix) ActiveProblems(hostID string) ([]zabbix.Problem, error) {
	if hostID == f.failOn {
		return nil, errors.New("zabbix fora")
	}
	return f.problems[hostID], nil
}

func writeDeps(t *testing.T, y string) config.RuntimeConfig {
	t.Helper()
	path := filepath.Join(t.TempDir(), "deps.yaml")
	if err := os.WriteFile(path, []byte(y), 0644); err != nil {
		t.Fatal(err)
	}
	return config.RuntimeConfig{DependenciesFile: path}
}

const twoParents = `
dependencies:
  - host: SRV-01
    hostid: "10"
    mode: %s
    parents:
      - host: SW-01
        hostid: "20"
        event: "Unavailable by ICMP ping"
      - host: SW-02
`

func depsYAML(mode string) string { return strings.Replace(twoParents, "%s", mode, 1) }

var fz = fakeZabbix{
	hosts: map[string]zabbix.Host{"sw-02": {HostID: "30", Host: "SW-02"}},
}

func TestCheckDependencyAnyHoldsWhenOneParentDown(t *testing.T) {
	cfg := writeDeps(t, depsYAML("any"))
	zx := fz
	zx.problems = map[string][]zabbix.Problem{
		"20": {{EventID: "500", Name: "unavailable by icmp ping"}},
	}

	msg, hold := checkDependency(cfg, zx, rawdata.Payload{Host: "SRV-01", HostID: "10"})
	if !hold {
		t.Fatal("esperava segurar o alerta")
	}
	if !strings.Contains(msg, "SW-01") || !strings.Contains(msg, "500") {
		t.Fatalf("mensagem sem o pai/evento: %q", msg)
	}
}

func TestCheckDependencyIgnoresProblemThatDoesNotMatchEvent(t *testing.T) {
	cfg := writeDeps(t, depsYAML("any"))
	zx := fz
	zx.problems = map[string][]zabbix.Problem{
		"20": {{EventID: "500", Name: "High CPU utilization"}},
	}

	if _, hold := checkDependency(cfg, zx, rawdata.Payload{Host: "SRV-01", HostID: "10"}); hold {
		t.Fatal("CPU alta no pai não deveria segurar o alerta")
	}
}

func TestCheckDependencyAllNeedsEveryParentDown(t *testing.T) {
	cfg := writeDeps(t, depsYAML("all"))
	zx := fz
	zx.problems = map[string][]zabbix.Problem{
		"20": {{EventID: "500", Name: "Unavailable by ICMP ping"}},
	}

	if _, hold := checkDependency(cfg, zx, rawdata.Payload{Host: "SRV-01", HostID: "10"}); hold {
		t.Fatal("mode=all com só um pai fora não deveria segurar")
	}

	// SW-02 resolvido pelo nome (sem hostid no YAML), qualquer problema conta
	zx.problems["30"] = []zabbix.Problem{{EventID: "501", Name: "Interface Gi0/1 down"}}
	if _, hold := checkDependency(cfg, zx, rawdata.Payload{Host: "SRV-01", HostID: "10"}); !hold {
		t.Fatal("mode=all com os dois pais fora deveria segurar")
	}
}

func TestCheckDependencyFailsOpen(t *testing.T) {
	cfg := writeDeps(t, depsYAML("any"))
	zx := fz
	zx.failOn = "20"

	if _, hold := checkDependency(cfg, zx, rawdata.Payload{Host: "SRV-01", HostID: "10"}); hold {
		t.Fatal("erro no Zabbix deveria abrir chamado normalmente")
	}
}

func TestCheckDependencyHostWithoutDependency(t *testing.T) {
	cfg := writeDeps(t, depsYAML("any"))
	if _, hold := checkDependency(cfg, fz, rawdata.Payload{Host: "OUTRO", HostID: "99"}); hold {
		t.Fatal("host sem dependência não deveria segurar")
	}
}

func TestMatchEvent(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          bool
	}{
		{"", "qualquer coisa", true},
		{"Unavailable by ICMP ping", "unavailable by icmp ping", true},
		{"Unavailable by ICMP ping", "Unavailable by ICMP ping (loss)", false},
		{"*Interface*down*", "Interface Gi0/1(uplink): Link down", true},
		{"*Interface*down*", "High CPU", false},
		{"Link (a.b)*", "Link (a.b) caiu", true},
		{"Link (a.b)*", "Link (aXb) caiu", false},
	}
	for _, c := range cases {
		if got := matchEvent(c.pattern, c.name); got != c.want {
			t.Errorf("matchEvent(%q, %q) = %v, esperava %v", c.pattern, c.name, got, c.want)
		}
	}
}
