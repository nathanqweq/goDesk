package app

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"godesk/internal/config"
	"godesk/internal/rawdata"
	"godesk/internal/zabbix"
)

// dependencyZabbix é o que checkDependency precisa do Zabbix (interface
// pra poder testar sem API).
type dependencyZabbix interface {
	HostsByName(names []string) (map[string]zabbix.Host, error)
	ActiveProblems(hostID string) ([]zabbix.Problem, error)
}

// parentHit é o resultado da consulta de um host pai: Problem != nil
// quando ele tem problema ativo que bate com o filtro de evento.
type parentHit struct {
	Parent  config.DependencyParent
	Problem *zabbix.Problem
}

// checkDependency decide se o alerta deve ser segurado (sem abrir chamado)
// porque o host depende de outro que já está em alerta. Qualquer falha
// (arquivo inválido, Zabbix fora, pai inexistente) conta como "pai não
// está em alerta": na dúvida, abre chamado — melhor um chamado a mais do
// que um alerta perdido.
func checkDependency(cfg config.RuntimeConfig, zx dependencyZabbix, p rawdata.Payload) (msg string, hold bool) {
	df, err := config.LoadDependencies(cfg.DependenciesFile)
	if err != nil {
		log.Printf("[dependency] WARN: falha ao ler %s, seguindo sem dependência: %v\n", cfg.DependenciesFile, err)
		return "", false
	}

	dep, ok := df.Find(p.HostID, p.Host)
	if !ok {
		log.Printf("[dependency] host=%q host_id=%q sem dependência cadastrada\n", p.Host, p.HostID)
		return "", false
	}

	hits := queryParents(zx, dep)
	msg, hold = evaluateDependency(p.Host, dep, hits)
	log.Printf("[dependency] host=%q mode=%s hold=%v %s\n", p.Host, dep.Mode, hold, describeHits(hits))
	return msg, hold
}

func queryParents(zx dependencyZabbix, dep config.Dependency) []parentHit {
	// pais sem hostid (ex: YAML escrito à mão) são resolvidos pelo nome
	var names []string
	for _, par := range dep.Parents {
		if par.HostID == "" {
			names = append(names, par.Host)
		}
	}
	resolved := map[string]zabbix.Host{}
	if len(names) > 0 {
		var err error
		resolved, err = zx.HostsByName(names)
		if err != nil {
			log.Printf("[dependency] WARN: falha ao resolver hosts pai %v: %v\n", names, err)
		}
	}

	hits := make([]parentHit, 0, len(dep.Parents))
	for _, par := range dep.Parents {
		hit := parentHit{Parent: par}

		hostID := par.HostID
		if hostID == "" {
			hostID = resolved[strings.ToLower(par.Host)].HostID
		}
		if hostID == "" {
			log.Printf("[dependency] WARN: host pai %q não encontrado no Zabbix (dependente=%q)\n", par.Host, dep.Host)
			hits = append(hits, hit)
			continue
		}

		problems, err := zx.ActiveProblems(hostID)
		if err != nil {
			log.Printf("[dependency] WARN: falha ao consultar problemas do pai %q: %v\n", par.Host, err)
			hits = append(hits, hit)
			continue
		}
		for i := range problems {
			if matchEvent(par.Event, problems[i].Name) {
				hit.Problem = &problems[i]
				break
			}
		}
		hits = append(hits, hit)
	}
	return hits
}

// evaluateDependency aplica o mode: any segura se pelo menos um pai está
// em alerta; all só segura se todos estiverem (redundância).
func evaluateDependency(host string, dep config.Dependency, hits []parentHit) (string, bool) {
	var down []parentHit
	for _, h := range hits {
		if h.Problem != nil {
			down = append(down, h)
		}
	}

	if len(down) == 0 {
		return "", false
	}
	if dep.Mode == config.DependencyModeAll && len(down) < len(hits) {
		return "", false
	}

	parts := make([]string, 0, len(down))
	for _, h := range down {
		parts = append(parts, fmt.Sprintf("%s (%q, evento %s)", h.Parent.Host, h.Problem.Name, h.Problem.EventID))
	}

	return fmt.Sprintf("goDesk: host %s depende de host(s) com alerta ativo: %s. Chamado não aberto.",
		host, strings.Join(parts, "; ")), true
}

func describeHits(hits []parentHit) string {
	parts := make([]string, 0, len(hits))
	for _, h := range hits {
		state := "ok"
		if h.Problem != nil {
			state = "alerta(evento " + h.Problem.EventID + ")"
		}
		parts = append(parts, h.Parent.Host+"="+state)
	}
	return "pais: " + strings.Join(parts, ", ")
}

// matchEvent compara o nome do problema com o filtro do pai: vazio = qualquer
// problema; sem "*" = nome exato; com "*" = curinga. Sempre case-insensitive.
func matchEvent(pattern, name string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return strings.EqualFold(pattern, strings.TrimSpace(name))
	}

	parts := strings.Split(pattern, "*")
	for i, part := range parts {
		parts[i] = regexp.QuoteMeta(part)
	}
	re, err := regexp.Compile("(?is)^" + strings.Join(parts, ".*") + "$")
	if err != nil {
		return false
	}
	return re.MatchString(strings.TrimSpace(name))
}
