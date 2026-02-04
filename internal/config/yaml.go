package config

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type TagsDefaults struct {
	Contract        string `yaml:"contract"`
	OperGroup       string `yaml:"oper_group"`
	MainCaller      string `yaml:"main_caller"`
	SecundaryCaller string `yaml:"secundary_caller"`
	// Cc string `yaml:"cc"` // se quiser usar depois
}

type Policy struct {
	Urgency   string       `yaml:"urgency"`
	Impact    string       `yaml:"impact"`
	AutoClose bool         `yaml:"autoclose"`
	Tags      TagsDefaults `yaml:"tags"`
}

type PoliciesFile struct {
	Default Policy            `yaml:"default"`
	Clients map[string]Policy `yaml:"clients"`
}

func LoadPolicies(path string) (PoliciesFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return PoliciesFile{}, err
	}

	var pf PoliciesFile
	if err := yaml.Unmarshal(b, &pf); err != nil {
		return PoliciesFile{}, err
	}

	// defaults safe
	if strings.TrimSpace(pf.Default.Urgency) == "" {
		pf.Default.Urgency = "Baixa"
	}
	if strings.TrimSpace(pf.Default.Impact) == "" {
		pf.Default.Impact = "Sem impacto"
	}
	if pf.Clients == nil {
		pf.Clients = map[string]Policy{}
	}

	return pf, nil
}

func ResolvePolicy(pf PoliciesFile, cliente string) Policy {
	cliente = strings.TrimSpace(cliente)
	p := pf.Default
	if cliente == "" {
		return p
	}

	// match exato
	if cpol, ok := pf.Clients[cliente]; ok {
		return mergePolicy(p, cpol)
	}

	// match case-insensitive
	for k, v := range pf.Clients {
		if strings.EqualFold(strings.TrimSpace(k), cliente) {
			return mergePolicy(p, v)
		}
	}

	return p
}

func mergePolicy(def Policy, over Policy) Policy {
	if strings.TrimSpace(over.Urgency) != "" {
		def.Urgency = over.Urgency
	}
	if strings.TrimSpace(over.Impact) != "" {
		def.Impact = over.Impact
	}
	// autoclose do cliente manda (se existir bloco do cliente)
	def.AutoClose = over.AutoClose

	// tags: só sobrescreve se cliente tiver valor
	if strings.TrimSpace(over.Tags.Contract) != "" {
		def.Tags.Contract = over.Tags.Contract
	}
	if strings.TrimSpace(over.Tags.OperGroup) != "" {
		def.Tags.OperGroup = over.Tags.OperGroup
	}
	if strings.TrimSpace(over.Tags.MainCaller) != "" {
		def.Tags.MainCaller = over.Tags.MainCaller
	}
	if strings.TrimSpace(over.Tags.SecundaryCaller) != "" {
		def.Tags.SecundaryCaller = over.Tags.SecundaryCaller
	}

	return def
}
