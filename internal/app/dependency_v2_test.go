package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"godesk/internal/config"
	"godesk/internal/rawdata"
	"godesk/internal/zabbix"
)

// ActiveProblemsByHost completa o fakeZabbix (dependency_test.go) pra
// também servir à v2.
func (f fakeZabbix) ActiveProblemsByHost(hostIDs []string) (map[string][]zabbix.Problem, error) {
	out := map[string][]zabbix.Problem{}
	for _, id := range hostIDs {
		if id == f.failOn {
			return nil, errors.New("zabbix fora")
		}
		if p, ok := f.problems[id]; ok {
			out[id] = p
		}
	}
	return out, nil
}

// Arquitetura da UHCC (mesma do diagrama): SDs → core em stack → FWs com
// perna nos dois membros → SW SCI por rack (interligados) → RTAC.
const uhccV2 = `
sites:
  - name: UHCC
    status:
      host: "UHCC - Core Status"
      event: "Powerplant Down"
    nodes:
      - {host: SD_UHCC_ADM, rack: "1"}
      - {host: SD_UHCC_OPE, rack: "2"}
      - {host: SW_CORE_UHCC_1, rack: "1"}
      - {host: SW_CORE_UHCC_2, rack: "2"}
      - {host: FW_UHCC_SCI_1, rack: "1"}
      - {host: FW_UHCC_SCI_2, rack: "2"}
      - {host: SW_UHCC_SCI_04, rack: "1"}
      - {host: SW_UHCC_SCI_05, rack: "2"}
      - {host: RTAC_UHCC_1, rack: "1"}
      - {host: RTAC_UHCC_2, rack: "2"}
    links:
      - [ZABBIX, SD_UHCC_ADM]
      - [ZABBIX, SD_UHCC_OPE]
      - [SD_UHCC_ADM, SW_CORE_UHCC_1]
      - [SD_UHCC_OPE, SW_CORE_UHCC_2]
      - [SW_CORE_UHCC_1, SW_CORE_UHCC_2]
      - [SW_CORE_UHCC, SW_CORE_UHCC_1]
      - [SW_CORE_UHCC, SW_CORE_UHCC_2]
      - [SW_CORE_UHCC_1, FW_UHCC_SCI_1]
      - [SW_CORE_UHCC_2, FW_UHCC_SCI_1]
      - [SW_CORE_UHCC_1, FW_UHCC_SCI_2]
      - [SW_CORE_UHCC_2, FW_UHCC_SCI_2]
      - [FW_UHCC_SCI_VIP, FW_UHCC_SCI_1]
      - [FW_UHCC_SCI_VIP, FW_UHCC_SCI_2]
      - [FW_UHCC_SCI_1, SW_UHCC_SCI_04]
      - [FW_UHCC_SCI_2, SW_UHCC_SCI_05]
      - [SW_UHCC_SCI_04, SW_UHCC_SCI_05]
      - [SW_UHCC_SCI_04, RTAC_UHCC_1]
      - [SW_UHCC_SCI_05, RTAC_UHCC_2]
`

var uhccHosts = []string{
	"UHCC - Core Status", "SD_UHCC_ADM", "SD_UHCC_OPE", "SW_CORE_UHCC", "SW_CORE_UHCC_1",
	"SW_CORE_UHCC_2", "FW_UHCC_SCI_1", "FW_UHCC_SCI_2", "FW_UHCC_SCI_VIP",
	"SW_UHCC_SCI_04", "SW_UHCC_SCI_05", "RTAC_UHCC_1", "RTAC_UHCC_2",
}

// uhccZabbix monta um fake com todos os hosts da UHCC (hostid = "id-" +
// nome) e os problemas informados: down recebe "Unavailable by ICMP ping",
// e siteDown liga o "Powerplant Down" no host de status.
func uhccZabbix(siteDown bool, down ...string) fakeZabbix {
	f := fakeZabbix{hosts: map[string]zabbix.Host{}, problems: map[string][]zabbix.Problem{}}
	for _, h := range uhccHosts {
		f.hosts[strings.ToLower(h)] = zabbix.Host{HostID: "id-" + h, Host: h}
	}
	for i, h := range down {
		f.problems["id-"+h] = []zabbix.Problem{{EventID: fmt.Sprint(100 + i), Name: "ICMP Ping: Unavailable by ICMP ping"}}
	}
	if siteDown {
		f.problems["id-UHCC - Core Status"] = []zabbix.Problem{{EventID: "999", Name: "Powerplant Down"}}
	}
	return f
}

func v2Config(t *testing.T) config.RuntimeConfig {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "deps-v2.yaml")
	if err := os.WriteFile(path, []byte(uhccV2), 0644); err != nil {
		t.Fatal(err)
	}
	return config.RuntimeConfig{
		DependenciesV2File: path,
		DependenciesFile:   filepath.Join(dir, "nao-existe.yaml"),
		MetricsFile:        filepath.Join(dir, "metrics.json"),
	}
}

func decideV2(t *testing.T, zx fakeZabbix, host string) v2Decision {
	t.Helper()
	return checkDependencyV2(v2Config(t), zx, rawdata.Payload{Host: host, EventID: "1"})
}

// Incidente real de 13/11 09:45: a usina inteira caiu e saíram 5 chamados
// (CORE_1, CORE_2, CORE, SD_ADM e o Powerplant Down). Com a v2, só o host
// de status abre.
func TestV2PowerplantDownHoldsEverythingButStatusHost(t *testing.T) {
	all := []string{"SD_UHCC_ADM", "SD_UHCC_OPE", "SW_CORE_UHCC", "SW_CORE_UHCC_1", "SW_CORE_UHCC_2",
		"FW_UHCC_SCI_1", "FW_UHCC_SCI_2", "FW_UHCC_SCI_VIP", "SW_UHCC_SCI_04", "SW_UHCC_SCI_05"}
	zx := uhccZabbix(true, all...)

	for _, h := range all {
		d := decideV2(t, zx, h)
		if !d.Hold || d.Reason != v2ReasonSiteDown {
			t.Errorf("%s: esperado hold por localidade_down, veio %s", h, d)
		}
		if !strings.Contains(d.Msg, "Powerplant Down") {
			t.Errorf("%s: mensagem sem o evento de status: %q", h, d.Msg)
		}
	}

	if d := decideV2(t, zx, "UHCC - Core Status"); d.Hold || d.Reason != v2ReasonStatusHost {
		t.Fatalf("host de status deveria abrir chamado, veio %s", d)
	}
}

// Mesmo cenário sem o Powerplant Down ainda ativo (status detectado
// depois): a topologia sozinha segura todo mundo atrás dos SDs.
func TestV2AllDownWithoutStatusOnlySDsAreRootCause(t *testing.T) {
	all := []string{"SD_UHCC_ADM", "SD_UHCC_OPE", "SW_CORE_UHCC_1", "SW_CORE_UHCC_2", "FW_UHCC_SCI_1", "FW_UHCC_SCI_2"}
	zx := uhccZabbix(false, all...)

	for _, h := range []string{"SD_UHCC_ADM", "SD_UHCC_OPE"} {
		if d := decideV2(t, zx, h); d.Hold {
			t.Errorf("%s é causa raiz, veio %s", h, d)
		}
	}
	// bloqueadores = primeiro corte no caminho a partir do root: os dois SDs
	d := decideV2(t, zx, "SW_CORE_UHCC_1")
	if !d.Hold || strings.Join(d.Blockers, ",") != "SD_UHCC_ADM,SD_UHCC_OPE" {
		t.Fatalf("CORE_1: esperado hold pelos dois SDs, veio %s", d)
	}
	if !strings.Contains(d.Msg, "SD_UHCC_ADM (rack 1)") {
		t.Fatalf("mensagem deveria trazer o rack: %q", d.Msg)
	}
}

// FW com perna nos dois membros do stack: um membro caído não segura o FW.
func TestV2RedundantParentsOnlyHoldWhenAllDown(t *testing.T) {
	if d := decideV2(t, uhccZabbix(false, "SW_CORE_UHCC_1"), "FW_UHCC_SCI_1"); d.Hold {
		t.Fatalf("um membro do core caído não deveria segurar o FW: %s", d)
	}
	d := decideV2(t, uhccZabbix(false, "SW_CORE_UHCC_1", "SW_CORE_UHCC_2"), "FW_UHCC_SCI_1")
	if !d.Hold || strings.Join(d.Blockers, ",") != "SW_CORE_UHCC_1,SW_CORE_UHCC_2" {
		t.Fatalf("os dois membros caídos deveriam segurar o FW, veio %s", d)
	}
	// IP do stack só cai junto com os dois membros
	if d := decideV2(t, uhccZabbix(false, "SW_CORE_UHCC_1", "SW_CORE_UHCC_2"), "SW_CORE_UHCC"); !d.Hold {
		t.Fatalf("IP do stack com os dois membros caídos é consequência: %s", d)
	}
}

// FW_SCI_1 caído: o SW_SCI_04 continua alcançável pelo SW_SCI_05, então
// se ele também cair é causa raiz (não é segurado pelo FW).
func TestV2SCISwitchReachableThroughOtherRack(t *testing.T) {
	if d := decideV2(t, uhccZabbix(false, "FW_UHCC_SCI_1"), "SW_UHCC_SCI_04"); d.Hold {
		t.Fatalf("SW_SCI_04 tem caminho pelo SW_SCI_05, deveria abrir: %s", d)
	}
	if d := decideV2(t, uhccZabbix(false, "FW_UHCC_SCI_1"), "FW_UHCC_SCI_1"); d.Hold {
		t.Fatalf("FW_SCI_1 é causa raiz: %s", d)
	}
}

// Caso misto entre racks: SW_SCI_04 e RTAC_UHCC_2 caídos. O RTAC_1 cai
// junto por causa do switch; o RTAC_2 é falha própria.
func TestV2MixedRackCase(t *testing.T) {
	zx := uhccZabbix(false, "SW_UHCC_SCI_04", "RTAC_UHCC_1", "RTAC_UHCC_2")

	d := decideV2(t, zx, "RTAC_UHCC_1")
	if !d.Hold || strings.Join(d.Blockers, ",") != "SW_UHCC_SCI_04" {
		t.Fatalf("RTAC_1 deveria ser segurado pelo SW_SCI_04, veio %s", d)
	}
	if d := decideV2(t, zx, "RTAC_UHCC_2"); d.Hold {
		t.Fatalf("RTAC_2 tem o SW_SCI_05 UP, é causa raiz: %s", d)
	}
	if d := decideV2(t, zx, "SW_UHCC_SCI_04"); d.Hold {
		t.Fatalf("SW_SCI_04 é causa raiz: %s", d)
	}
}

// Problema que não é de ping (ex: CPU alta) não conta como DOWN.
func TestV2OnlyDownEventCountsAsDown(t *testing.T) {
	zx := uhccZabbix(false)
	zx.problems["id-SW_UHCC_SCI_04"] = []zabbix.Problem{{EventID: "5", Name: "High CPU utilization"}}
	if d := decideV2(t, zx, "RTAC_UHCC_1"); d.Hold {
		t.Fatalf("CPU alta no pai não deveria segurar: %s", d)
	}
}

// Pedaço da topologia sem ligação com o root (desenho incompleto): nenhum
// DOWN explica o corte, então não pode segurar.
func TestV2DisconnectedTopologyOpensTicket(t *testing.T) {
	site := config.Site{
		Name: "X", Root: "ZABBIX",
		Links: [][]string{{"ZABBIX", "SD"}, {"ILHA_A", "ILHA_B"}},
	}
	d := evaluateV2(site, "ILHA_A", siteStatus{Down: map[string]bool{"ilha_b": true}})
	if d.Hold {
		t.Fatalf("host sem caminho até o root no desenho não deveria ser segurado: %s", d)
	}
}

func TestParseDependenciesV2Validation(t *testing.T) {
	cases := map[string]string{
		"host em duas localidades": `
sites:
  - name: A
    links: [[ZABBIX, SW1]]
  - name: B
    links: [[ZABBIX, SW1]]`,
		"root sem link": `
sites:
  - name: A
    links: [[SW1, SW2]]`,
		"link com 3 hosts": `
sites:
  - name: A
    links: [[ZABBIX, SW1, SW2]]`,
		"status nos links": `
sites:
  - name: A
    status: {host: ST}
    links: [[ZABBIX, ST]]`,
		"localidade repetida": `
sites:
  - name: A
  - name: a`,
	}
	for name, y := range cases {
		if _, err := config.ParseDependenciesV2([]byte(y)); err == nil {
			t.Errorf("%s: esperado erro de validação", name)
		}
	}

	f, err := config.ParseDependenciesV2([]byte(uhccV2))
	if err != nil {
		t.Fatalf("arquitetura da UHCC deveria ser válida: %v", err)
	}
	s := f.Sites[0]
	if s.Root != config.DefaultTopologyRoot || s.DownEvent != config.DefaultTopologyDownEvent {
		t.Fatalf("defaults não aplicados: root=%q down_event=%q", s.Root, s.DownEvent)
	}
	if _, ok := f.SiteFor("uhcc - core status"); !ok {
		t.Fatal("host de status deveria pertencer à localidade (case-insensitive)")
	}
}

func TestV2HostOutsideSitesIsNotManaged(t *testing.T) {
	if d := decideV2(t, uhccZabbix(false), "SRV-QUALQUER"); d.Managed {
		t.Fatalf("host fora da v2 não deveria ser gerenciado: %s", d)
	}
}

func TestV2ZabbixErrorOpensTicket(t *testing.T) {
	zx := uhccZabbix(true, "SW_UHCC_SCI_04")
	zx.failOn = "id-SW_UHCC_SCI_04"
	if d := decideV2(t, zx, "RTAC_UHCC_1"); d.Hold || d.Reason != v2ReasonError {
		t.Fatalf("erro no Zabbix deveria abrir chamado, veio %s", d)
	}
}

func TestRunDependencyVersions(t *testing.T) {
	cfg := v2Config(t)
	zx := uhccZabbix(true, "SW_CORE_UHCC_1")
	p := rawdata.Payload{Host: "SW_CORE_UHCC_1", EventID: "1"}

	// v1 sem cadastro: não segura
	if _, hold := runDependency(cfg, zx, p, ""); hold {
		t.Fatal("v1 sem cadastro não deveria segurar")
	}
	// shadow: quem decide é a v1, mesmo a v2 querendo segurar
	if _, hold := runDependency(cfg, zx, p, "shadow"); hold {
		t.Fatal("shadow não pode mudar a decisão da v1")
	}
	// v2: segura pela localidade
	msg, hold := runDependency(cfg, zx, p, "v2")
	if !hold || !strings.Contains(msg, "dependência v2") {
		t.Fatalf("v2 deveria segurar, veio hold=%v msg=%q", hold, msg)
	}
	// v2 com host fora das localidades cai na v1
	if _, hold := runDependency(cfg, zx, rawdata.Payload{Host: "SRV-01"}, "v2"); hold {
		t.Fatal("host fora da v2 deveria cair na v1 (sem cadastro = não segura)")
	}
}
