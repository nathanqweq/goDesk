package app

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"godesk/internal/config"
	"godesk/internal/metrics"
	"godesk/internal/rawdata"
	"godesk/internal/zabbix"
)

// Dependência v2: arquitetura por localidade (godesk-dependencies-v2.yaml).
// Roda em paralelo à v1, escolhida por rule (dependency_version):
//
//   - v1:     só a v1 (comportamento de sempre);
//   - v2:     a v2 decide; host fora de qualquer localidade da v2 cai na v1;
//   - shadow: a v1 decide e a v2 roda junto só pra registrar no log (e na
//     métrica de divergência) o que ela teria feito — pra testar a v2 em
//     produção sem risco.
//
// A decisão da v2, em ordem:
//  1. host fora de qualquer localidade         → não gerenciado
//  2. host é o próprio host de status          → abre (é o chamado da usina)
//  3. evento de status ativo (Powerplant Down) → segura
//  4. host fora dos links da topologia         → abre
//  5. sem caminho de root até um vizinho do host passando só por hosts UP
//     (BFS)                                    → segura, com os bloqueadores
//  6. caso contrário                           → abre (é causa raiz)
//
// Igual à v1: qualquer erro (arquivo inválido, Zabbix fora) abre chamado.

type dependencyV2Zabbix interface {
	HostsByName(names []string) (map[string]zabbix.Host, error)
	ActiveProblemsByHost(hostIDs []string) (map[string][]zabbix.Problem, error)
}

// Motivos da decisão v2 (aparecem no log e na mensagem do evento).
const (
	v2ReasonNotManaged  = "fora_da_v2"
	v2ReasonStatusHost  = "host_de_status"
	v2ReasonSiteDown    = "localidade_down"
	v2ReasonNotInLinks  = "fora_da_topologia"
	v2ReasonUnreachable = "inalcancavel"
	v2ReasonRootCause   = "causa_raiz"
	v2ReasonError       = "erro"
)

type v2Decision struct {
	Managed  bool
	Hold     bool
	Site     string
	Reason   string
	Blockers []string
	Msg      string
}

func (d v2Decision) String() string {
	s := fmt.Sprintf("site=%q hold=%v motivo=%s", d.Site, d.Hold, d.Reason)
	if len(d.Blockers) > 0 {
		s += " bloqueadores=" + strings.Join(d.Blockers, ",")
	}
	return s
}

// runDependency escolhe a versão da dependência pela rule e devolve a
// decisão final (msg do comentário no evento, hold).
func runDependency(cfg config.RuntimeConfig, zx interface {
	dependencyZabbix
	dependencyV2Zabbix
}, p rawdata.Payload, version string) (string, bool) {
	switch config.NormalizeDependencyVersion(version) {
	case config.DependencyVersionV2:
		d := checkDependencyV2(cfg, zx, p)
		if !d.Managed {
			log.Printf("[dependency-v2] host=%q fora das localidades da v2, usando v1\n", p.Host)
			return checkDependency(cfg, zx, p)
		}
		recordMetric(cfg.MetricsFile, func(m *metrics.Snapshot) {
			m.DependencyV2Evaluated++
			if d.Hold {
				m.DependencyV2Held++
			}
		})
		return d.Msg, d.Hold

	case config.DependencyVersionShadow:
		msg, hold := checkDependency(cfg, zx, p)
		d := checkDependencyV2(cfg, zx, p)
		if d.Managed {
			diverge := d.Hold != hold
			tag := "IGUAL"
			if diverge {
				tag = "DIVERGE"
			}
			log.Printf("[dependency-shadow] %s host=%q event=%s v1_hold=%v v2_hold=%v %s\n",
				tag, p.Host, p.EventID, hold, d.Hold, d)
			recordMetric(cfg.MetricsFile, func(m *metrics.Snapshot) {
				m.DependencyShadowEvaluated++
				if diverge {
					m.DependencyShadowDiverged++
				}
			})
		} else {
			log.Printf("[dependency-shadow] host=%q fora das localidades da v2\n", p.Host)
		}
		return msg, hold

	default:
		return checkDependency(cfg, zx, p)
	}
}

// checkDependencyV2 carrega o arquivo, consulta o estado da localidade no
// Zabbix e aplica evaluateV2.
func checkDependencyV2(cfg config.RuntimeConfig, zx dependencyV2Zabbix, p rawdata.Payload) v2Decision {
	f, err := config.LoadDependenciesV2(cfg.DependenciesV2File)
	if err != nil {
		log.Printf("[dependency-v2] WARN: falha ao ler %s, seguindo sem dependência: %v\n", cfg.DependenciesV2File, err)
		return v2Decision{Reason: v2ReasonError}
	}

	site, ok := f.SiteFor(p.Host)
	if !ok {
		return v2Decision{Reason: v2ReasonNotManaged}
	}

	state, err := siteState(zx, site, p.Host)
	if err != nil {
		log.Printf("[dependency-v2] WARN: falha ao consultar estado da localidade %q, abrindo chamado: %v\n", site.Name, err)
		return v2Decision{Managed: true, Site: site.Name, Reason: v2ReasonError}
	}

	d := evaluateV2(site, p.Host, state)
	log.Printf("[dependency-v2] host=%q %s down=%v\n", p.Host, d, state.downList())
	return d
}

// siteStatus é o estado da localidade no momento do alerta.
type siteStatus struct {
	// Down: hosts da topologia (lowercase) com problema que bate com
	// site.DownEvent.
	Down map[string]bool
	// StatusProblem: problema do host de status que bate com
	// site.Status.Event (nil = localidade não está DOWN).
	StatusProblem *zabbix.Problem
	// display: lowercase → nome como está no arquivo (só pra log/mensagem)
	display map[string]string
}

func (s siteStatus) downList() []string {
	var out []string
	for k := range s.Down {
		out = append(out, s.display[k])
	}
	sort.Strings(out)
	return out
}

func siteState(zx dependencyV2Zabbix, site config.Site, alertHost string) (siteStatus, error) {
	st := siteStatus{Down: map[string]bool{}, display: map[string]string{}}

	names := site.TopologyHosts()
	if site.Status.Host != "" {
		names = append(names, site.Status.Host)
	}
	for _, n := range names {
		st.display[strings.ToLower(n)] = n
	}

	resolved, err := zx.HostsByName(names)
	if err != nil {
		return st, err
	}

	var ids []string
	nameOf := map[string]string{} // hostid → lowercase do nome no arquivo
	for _, n := range names {
		h, ok := resolved[strings.ToLower(n)]
		if !ok || h.HostID == "" {
			// host que não existe no Zabbix conta como UP: não bloqueia
			// ninguém (na dúvida, abre chamado)
			if !strings.EqualFold(n, alertHost) {
				log.Printf("[dependency-v2] WARN: host %q da localidade %q não encontrado no Zabbix (conta como UP)\n", n, site.Name)
			}
			continue
		}
		ids = append(ids, h.HostID)
		nameOf[h.HostID] = strings.ToLower(n)
	}

	problems, err := zx.ActiveProblemsByHost(ids)
	if err != nil {
		return st, err
	}

	statusKey := strings.ToLower(site.Status.Host)
	for hid, list := range problems {
		key, ok := nameOf[hid]
		if !ok {
			continue
		}
		for i := range list {
			if key == statusKey {
				if st.StatusProblem == nil && matchEvent(site.Status.Event, list[i].Name) {
					pr := list[i]
					st.StatusProblem = &pr
				}
				continue
			}
			if matchEvent(site.DownEvent, list[i].Name) {
				st.Down[key] = true
				break
			}
		}
	}
	return st, nil
}

// evaluateV2 é a decisão pura (sem I/O) — ver comentário no topo do arquivo.
func evaluateV2(site config.Site, host string, st siteStatus) v2Decision {
	d := v2Decision{Managed: true, Site: site.Name}

	if site.Status.Host != "" && strings.EqualFold(host, site.Status.Host) {
		d.Reason = v2ReasonStatusHost
		return d
	}

	if st.StatusProblem != nil {
		d.Hold = true
		d.Reason = v2ReasonSiteDown
		d.Msg = fmt.Sprintf("goDesk (dependência v2): localidade %s com %q ativo em %s (evento %s) — host %s é consequência. Chamado não aberto.",
			site.Name, st.StatusProblem.Name, site.Status.Host, st.StatusProblem.EventID, host)
		return d
	}

	topo := newTopology(site)
	if !topo.has(host) {
		d.Reason = v2ReasonNotInLinks
		return d
	}

	rootCause, blockers := topo.analyze(host, st.Down)
	if rootCause {
		d.Reason = v2ReasonRootCause
		return d
	}

	d.Hold = true
	d.Reason = v2ReasonUnreachable
	parts := make([]string, 0, len(blockers))
	for _, b := range blockers {
		name := topo.name[b]
		d.Blockers = append(d.Blockers, name)
		if rack := site.RackOf(name); rack != "" {
			name += " (rack " + rack + ")"
		}
		parts = append(parts, name)
	}
	d.Msg = fmt.Sprintf("goDesk (dependência v2): host %s inalcançável na arquitetura de %s — caminho cortado por: %s. Chamado não aberto.",
		host, site.Name, strings.Join(parts, ", "))
	return d
}

// topology é o grafo não-direcionado da localidade, com chaves em
// lowercase (o nome original fica em name).
type topology struct {
	root string
	adj  map[string]map[string]bool
	name map[string]string
}

func newTopology(site config.Site) topology {
	t := topology{
		root: strings.ToLower(site.Root),
		adj:  map[string]map[string]bool{},
		name: map[string]string{},
	}
	for _, l := range site.Links {
		a, b := strings.ToLower(l[0]), strings.ToLower(l[1])
		t.name[a], t.name[b] = l[0], l[1]
		if t.adj[a] == nil {
			t.adj[a] = map[string]bool{}
		}
		if t.adj[b] == nil {
			t.adj[b] = map[string]bool{}
		}
		t.adj[a][b] = true
		t.adj[b][a] = true
	}
	return t
}

func (t topology) has(host string) bool {
	k := strings.ToLower(host)
	return k != t.root && len(t.adj[k]) > 0
}

// analyze: host é causa raiz se ainda existe caminho de root até algum
// vizinho dele passando só por hosts UP. Se não existe, devolve os
// bloqueadores: os hosts DOWN na fronteira entre a parte alcançável e o
// "lado" do host (descarta DOWNs sem relação com o caminho dele).
func (t topology) analyze(host string, down map[string]bool) (bool, []string) {
	h := strings.ToLower(host)

	visited := map[string]bool{t.root: true}
	frontier := map[string]bool{}
	queue := []string{t.root}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for v := range t.adj[cur] {
			if v == h || visited[v] {
				continue
			}
			if down[v] {
				frontier[v] = true
				continue
			}
			visited[v] = true
			queue = append(queue, v)
		}
	}

	for v := range t.adj[h] {
		if visited[v] {
			return true, nil
		}
	}

	// lado do host: tudo que dá pra alcançar a partir dele sem entrar na
	// parte alcançável; os bloqueadores da fronteira não são expandidos
	side := map[string]bool{h: true}
	queue = []string{h}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for v := range t.adj[cur] {
			if visited[v] || side[v] {
				continue
			}
			side[v] = true
			if !frontier[v] {
				queue = append(queue, v)
			}
		}
	}

	var blockers []string
	for v := range frontier {
		if side[v] {
			blockers = append(blockers, v)
		}
	}
	// nenhum host DOWN explica o corte: a topologia é que não liga o host
	// ao root (desenho incompleto) — na dúvida, abre chamado
	if len(blockers) == 0 {
		return true, nil
	}
	sort.Strings(blockers)
	return false, blockers
}
