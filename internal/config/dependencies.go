package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// godesk-dependencies.yaml — arquivo separado do godesk-config.yaml,
// editado pela página "Dependências" do módulo Zabbix e sincronizado pelo
// mesmo caminho (godesk-client → POST /dependencies). Diz quais hosts
// dependem de quais: quando uma rule tem dependency: true e o host do
// alerta está aqui, o goDesk consulta os problemas ativos dos pais no
// Zabbix antes de abrir chamado (ver internal/app/dependency.go).
//
//	dependencies:
//	  - host: SRV-APP-01        # nome técnico do host dependente
//	    hostid: "10612"         # opcional — o módulo preenche ao salvar
//	    mode: any               # any (padrão) | all
//	    parents:
//	      - host: SW-CORE-01
//	        hostid: "10584"
//	        event: "Unavailable by ICMP ping"   # vazio = qualquer problema; aceita *
const (
	DependencyModeAny = "any"
	DependencyModeAll = "all"
)

type DependencyParent struct {
	Host   string `yaml:"host"`
	HostID string `yaml:"hostid"`
	// Event filtra quais problemas do pai contam: nome exato do problema
	// (case-insensitive) ou padrão com "*". Vazio = qualquer problema.
	Event string `yaml:"event"`
}

type Dependency struct {
	Host    string             `yaml:"host"`
	HostID  string             `yaml:"hostid"`
	Mode    string             `yaml:"mode"`
	Parents []DependencyParent `yaml:"parents"`
}

type DependenciesFile struct {
	Dependencies []Dependency `yaml:"dependencies"`
}

// LoadDependencies lê o arquivo de dependências. Arquivo inexistente não é
// erro — só significa que nenhuma dependência foi cadastrada ainda.
func LoadDependencies(path string) (DependenciesFile, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return DependenciesFile{}, nil
	}
	if err != nil {
		return DependenciesFile{}, err
	}
	return ParseDependencies(b)
}

// ParseDependencies faz o parse e valida o conteúdo (usado tanto por
// LoadDependencies quanto pelo POST /dependencies e pelo godesk-client
// antes de enviar): host obrigatório, pelo menos um pai, mode válido, sem
// host repetido, sem host dependendo de si mesmo e sem ciclo (A→B→A).
func ParseDependencies(data []byte) (DependenciesFile, error) {
	var df DependenciesFile
	if err := yaml.Unmarshal(data, &df); err != nil {
		return DependenciesFile{}, err
	}

	seen := map[string]bool{}
	graph := map[string][]string{}

	for i := range df.Dependencies {
		d := &df.Dependencies[i]
		d.Host = strings.TrimSpace(d.Host)
		d.HostID = strings.TrimSpace(d.HostID)
		d.Mode = strings.ToLower(strings.TrimSpace(d.Mode))

		if d.Host == "" {
			return DependenciesFile{}, fmt.Errorf("dependencies[%d]: host vazio", i)
		}
		switch d.Mode {
		case "":
			d.Mode = DependencyModeAny
		case DependencyModeAny, DependencyModeAll:
		default:
			return DependenciesFile{}, fmt.Errorf("host %q: mode %q inválido (use any ou all)", d.Host, d.Mode)
		}

		key := strings.ToLower(d.Host)
		if seen[key] {
			return DependenciesFile{}, fmt.Errorf("host %q cadastrado mais de uma vez", d.Host)
		}
		seen[key] = true

		if len(d.Parents) == 0 {
			return DependenciesFile{}, fmt.Errorf("host %q: nenhum host pai informado", d.Host)
		}
		for j := range d.Parents {
			p := &d.Parents[j]
			p.Host = strings.TrimSpace(p.Host)
			p.HostID = strings.TrimSpace(p.HostID)
			p.Event = strings.TrimSpace(p.Event)

			if p.Host == "" {
				return DependenciesFile{}, fmt.Errorf("host %q: pai #%d sem host", d.Host, j+1)
			}
			if strings.EqualFold(p.Host, d.Host) {
				return DependenciesFile{}, fmt.Errorf("host %q não pode depender de si mesmo", d.Host)
			}
			graph[key] = append(graph[key], strings.ToLower(p.Host))
		}
	}

	if cycle := findCycle(graph); cycle != "" {
		return DependenciesFile{}, fmt.Errorf("dependência circular: %s", cycle)
	}

	return df, nil
}

// Find devolve a dependência cadastrada pro host do alerta: primeiro por
// hostid (sobrevive a renomear o host), depois pelo nome técnico
// (case-insensitive — cobre arquivo importado de outro Zabbix, onde os
// hostids não batem).
func (df DependenciesFile) Find(hostID, host string) (Dependency, bool) {
	hostID = strings.TrimSpace(hostID)
	host = strings.TrimSpace(host)

	if hostID != "" {
		for _, d := range df.Dependencies {
			if d.HostID == hostID {
				return d, true
			}
		}
	}
	if host != "" {
		for _, d := range df.Dependencies {
			if strings.EqualFold(d.Host, host) {
				return d, true
			}
		}
	}
	return Dependency{}, false
}

func findCycle(graph map[string][]string) string {
	const (
		unvisited = iota
		visiting
		done
	)
	state := map[string]int{}
	var path []string

	var visit func(n string) string
	visit = func(n string) string {
		state[n] = visiting
		path = append(path, n)
		for _, next := range graph[n] {
			switch state[next] {
			case visiting:
				start := 0
				for i, p := range path {
					if p == next {
						start = i
						break
					}
				}
				return strings.Join(append(append([]string{}, path[start:]...), next), " → ")
			case unvisited:
				if c := visit(next); c != "" {
					return c
				}
			}
		}
		path = path[:len(path)-1]
		state[n] = done
		return ""
	}

	for n := range graph {
		if state[n] == unvisited {
			if c := visit(n); c != "" {
				return c
			}
		}
	}
	return ""
}
