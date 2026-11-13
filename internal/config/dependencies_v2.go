package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// godesk-dependencies-v2.yaml — dependência v2, em paralelo à v1
// (godesk-dependencies.yaml). Em vez de "host X depende dos pais Y, Z", a
// v2 descreve a ARQUITETURA de cada localidade:
//
//   - status: host agregado da localidade (ex: "UHCC - Core Status") e o
//     evento que significa "localidade inteira caiu" (ex: Powerplant Down).
//     Com ele ativo, nenhum host da localidade abre chamado — só ele.
//   - links: os cabos/caminhos por onde o monitoramento passa, a partir de
//     root (o ponto de entrada do Zabbix). Um host só abre chamado se ainda
//     existir caminho de root até algum vizinho dele passando apenas por
//     hosts UP (redundância de verdade: FW com perna nos dois cores só é
//     consequência se os dois caírem).
//   - nodes: posição (x/y) e rack de cada host, usados pelo editor visual
//     do módulo e para identificar o rack nas mensagens.
//
// Qual versão roda é escolhido por rule (dependency_version no
// godesk-config.yaml: v1, v2 ou shadow) — ver internal/app/dependency_v2.go.
//
//	sites:
//	  - name: UHCC
//	    root: ZABBIX                          # padrão ZABBIX
//	    down_event: "*Unavailable by ICMP ping*"
//	    status:
//	      host: "UHCC - Core Status"
//	      event: "Powerplant Down"            # vazio = qualquer problema
//	    nodes:
//	      - {host: SD_UHCC_ADM, rack: "1", x: 120, y: 80}
//	    links:
//	      - [ZABBIX, SD_UHCC_ADM]
//	      - [SD_UHCC_ADM, SW_CORE_UHCC_1]

const DefaultDependenciesV2File = "/etc/zabbix/godesk/godesk-dependencies-v2.yaml"

const (
	DefaultTopologyRoot      = "ZABBIX"
	DefaultTopologyDownEvent = "*Unavailable by ICMP ping*"
)

// Versões de dependência por rule (Policy.DependencyVersion).
const (
	DependencyVersionV1     = "v1"
	DependencyVersionV2     = "v2"
	DependencyVersionShadow = "shadow"
)

// NormalizeDependencyVersion: vazio ou desconhecido = v1 (comportamento de
// sempre — uma rule antiga, sem o campo, não muda nada).
func NormalizeDependencyVersion(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case DependencyVersionV2:
		return DependencyVersionV2
	case DependencyVersionShadow:
		return DependencyVersionShadow
	default:
		return DependencyVersionV1
	}
}

type SiteStatus struct {
	Host  string `yaml:"host"`
	Event string `yaml:"event"`
}

type TopologyNode struct {
	Host string `yaml:"host"`
	Rack string `yaml:"rack,omitempty"`
	X    int    `yaml:"x,omitempty"`
	Y    int    `yaml:"y,omitempty"`
}

type Site struct {
	Name      string         `yaml:"name"`
	Root      string         `yaml:"root"`
	DownEvent string         `yaml:"down_event"`
	Status    SiteStatus     `yaml:"status"`
	Nodes     []TopologyNode `yaml:"nodes"`
	Links     [][]string     `yaml:"links"`
}

type DependenciesV2File struct {
	Sites []Site `yaml:"sites"`
}

// LoadDependenciesV2: arquivo inexistente = nenhuma localidade cadastrada.
func LoadDependenciesV2(path string) (DependenciesV2File, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return DependenciesV2File{}, nil
	}
	if err != nil {
		return DependenciesV2File{}, err
	}
	return ParseDependenciesV2(b)
}

// ParseDependenciesV2 faz o parse, aplica defaults e valida: nome de
// localidade obrigatório e único, link com exatamente 2 hosts diferentes,
// nó sem host repetido e nenhum host em duas localidades (senão não dá pra
// saber qual arquitetura vale pro alerta). O root de cada localidade não
// conta como host — "ZABBIX" pode (e costuma) se repetir em todas.
func ParseDependenciesV2(data []byte) (DependenciesV2File, error) {
	var f DependenciesV2File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return DependenciesV2File{}, err
	}

	siteNames := map[string]bool{}
	owner := map[string]string{} // host (lower) → localidade

	claim := func(site, host string) error {
		key := strings.ToLower(host)
		if prev, ok := owner[key]; ok && prev != site {
			return fmt.Errorf("host %q está nas localidades %q e %q", host, prev, site)
		}
		owner[key] = site
		return nil
	}

	for i := range f.Sites {
		s := &f.Sites[i]
		s.Name = strings.TrimSpace(s.Name)
		s.Root = strings.TrimSpace(s.Root)
		s.DownEvent = strings.TrimSpace(s.DownEvent)
		s.Status.Host = strings.TrimSpace(s.Status.Host)
		s.Status.Event = strings.TrimSpace(s.Status.Event)

		if s.Name == "" {
			return DependenciesV2File{}, fmt.Errorf("sites[%d]: nome da localidade vazio", i)
		}
		if siteNames[strings.ToLower(s.Name)] {
			return DependenciesV2File{}, fmt.Errorf("localidade %q cadastrada mais de uma vez", s.Name)
		}
		siteNames[strings.ToLower(s.Name)] = true

		if s.Root == "" {
			s.Root = DefaultTopologyRoot
		}
		if s.DownEvent == "" {
			s.DownEvent = DefaultTopologyDownEvent
		}
		isRoot := func(h string) bool { return strings.EqualFold(h, s.Root) }

		if s.Status.Host != "" {
			if isRoot(s.Status.Host) {
				return DependenciesV2File{}, fmt.Errorf("localidade %q: host de status não pode ser o root %q", s.Name, s.Root)
			}
			if err := claim(s.Name, s.Status.Host); err != nil {
				return DependenciesV2File{}, err
			}
		}

		seenNode := map[string]bool{}
		for j := range s.Nodes {
			n := &s.Nodes[j]
			n.Host = strings.TrimSpace(n.Host)
			n.Rack = strings.TrimSpace(n.Rack)
			if n.Host == "" {
				return DependenciesV2File{}, fmt.Errorf("localidade %q: nó #%d sem host", s.Name, j+1)
			}
			key := strings.ToLower(n.Host)
			if seenNode[key] {
				return DependenciesV2File{}, fmt.Errorf("localidade %q: nó %q repetido", s.Name, n.Host)
			}
			seenNode[key] = true
			if !isRoot(n.Host) {
				if err := claim(s.Name, n.Host); err != nil {
					return DependenciesV2File{}, err
				}
			}
		}

		rootLinked := false
		for j := range s.Links {
			l := s.Links[j]
			if len(l) != 2 {
				return DependenciesV2File{}, fmt.Errorf("localidade %q: link #%d precisa de exatamente 2 hosts", s.Name, j+1)
			}
			l[0], l[1] = strings.TrimSpace(l[0]), strings.TrimSpace(l[1])
			if l[0] == "" || l[1] == "" {
				return DependenciesV2File{}, fmt.Errorf("localidade %q: link #%d com host vazio", s.Name, j+1)
			}
			if strings.EqualFold(l[0], l[1]) {
				return DependenciesV2File{}, fmt.Errorf("localidade %q: link #%d liga %q nele mesmo", s.Name, j+1, l[0])
			}
			for _, h := range l {
				if isRoot(h) {
					rootLinked = true
					continue
				}
				// o host de status é avaliado à parte (evento de status),
				// nunca como nó da topologia
				if s.Status.Host != "" && strings.EqualFold(h, s.Status.Host) {
					return DependenciesV2File{}, fmt.Errorf("localidade %q: o host de status %q não entra nos links", s.Name, h)
				}
				if err := claim(s.Name, h); err != nil {
					return DependenciesV2File{}, err
				}
			}
		}
		// sem link saindo do root nada é alcançável e TODO host da
		// localidade seria segurado — melhor recusar o arquivo
		if len(s.Links) > 0 && !rootLinked {
			return DependenciesV2File{}, fmt.Errorf("localidade %q: nenhum link sai do root %q (ligue o root aos equipamentos de entrada, ex: os SDs)", s.Name, s.Root)
		}
	}

	return f, nil
}

// SiteFor devolve a localidade do host (como nó, ponta de link ou host de
// status). Case-insensitive, igual ao Find da v1.
func (f DependenciesV2File) SiteFor(host string) (Site, bool) {
	host = strings.TrimSpace(host)
	if host == "" {
		return Site{}, false
	}
	for _, s := range f.Sites {
		if s.HasHost(host) {
			return s, true
		}
	}
	return Site{}, false
}

func (s Site) HasHost(host string) bool {
	if strings.EqualFold(host, s.Root) {
		return false
	}
	if strings.EqualFold(host, s.Status.Host) {
		return true
	}
	for _, n := range s.Nodes {
		if strings.EqualFold(n.Host, host) {
			return true
		}
	}
	for _, l := range s.Links {
		if strings.EqualFold(l[0], host) || strings.EqualFold(l[1], host) {
			return true
		}
	}
	return false
}

// TopologyHosts: todos os hosts da topologia (nós + pontas de link), sem o
// root e sem o host de status, com o nome como foi escrito no arquivo.
func (s Site) TopologyHosts() []string {
	seen := map[string]bool{strings.ToLower(s.Root): true}
	if s.Status.Host != "" {
		seen[strings.ToLower(s.Status.Host)] = true
	}
	var out []string
	add := func(h string) {
		if !seen[strings.ToLower(h)] {
			seen[strings.ToLower(h)] = true
			out = append(out, h)
		}
	}
	for _, n := range s.Nodes {
		add(n.Host)
	}
	for _, l := range s.Links {
		add(l[0])
		add(l[1])
	}
	return out
}

// RackOf devolve o rack cadastrado no nó (vazio se não tiver).
func (s Site) RackOf(host string) string {
	for _, n := range s.Nodes {
		if strings.EqualFold(n.Host, host) {
			return n.Rack
		}
	}
	return ""
}
