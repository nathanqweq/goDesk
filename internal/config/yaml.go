package config

import (
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type TopDeskDefaults struct {
	Contract        string `yaml:"contract"`
	Operator        string `yaml:"operator"`
	OperGroup       string `yaml:"oper_group"`
	MainCaller      string `yaml:"main_caller"`
	SecundaryCaller string `yaml:"secundary_caller"`
	Sla             string `yaml:"sla"`
	Category        string `yaml:"category"`
	SubCategory     string `yaml:"sub_category"`
	CallType        string `yaml:"call_type"`
	SendMoreInfo    bool   `yaml:"send_more_info"`
	MoreInfoText    string `yaml:"more_info_text"`
	AdicionalCresol bool   `yaml:"adicional_cresol"`
	SendEmail       bool   `yaml:"send_email"`
	EmailTo         string `yaml:"email_to"`
	EmailCc         string `yaml:"email_cc"`
}

type Policy struct {
	// nome bonito / display do cliente (não é a key do map)
	Client string `yaml:"client"`

	Priority  string `yaml:"priority"`
	Urgency   string `yaml:"urgency"`
	Impact    string `yaml:"impact"`
	AutoClose bool   `yaml:"autoclose"`

	TopDesk TopDeskDefaults `yaml:"topdesk"`
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

	return ParsePolicies(b)
}

// ParsePolicies faz o parse de um YAML de políticas já em memória (usado
// tanto por LoadPolicies quanto pelo handler POST /config do modo serviço,
// que recebe o conteúdo pela rede antes de gravar em disco).
func ParsePolicies(data []byte) (PoliciesFile, error) {
	var pf PoliciesFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
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

// SaveFile grava data em path de forma atômica (escreve em arquivo
// temporário e faz rename), preservando uma cópia do conteúdo anterior em
// "<path>.bak.<timestamp>" quando o arquivo já existir. Retorna o caminho
// do backup criado (vazio se não havia arquivo anterior).
func SaveFile(path string, data []byte) (string, error) {
	backupPath := ""

	if prev, err := os.ReadFile(path); err == nil {
		backupPath = path + ".bak." + time.Now().Format("20060102-150405")
		if err := os.WriteFile(backupPath, prev, 0644); err != nil {
			return "", err
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return "", err
	}

	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return "", err
	}

	return backupPath, nil
}

// ResolvePolicy agora recebe RULE NAME (key em clients:)
func ResolvePolicy(pf PoliciesFile, ruleName string) Policy {
	ruleName = strings.TrimSpace(ruleName)
	p := pf.Default
	if ruleName == "" {
		return p
	}

	// match exato
	if cpol, ok := pf.Clients[ruleName]; ok {
		return mergePolicy(p, cpol)
	}

	// match case-insensitive
	for k, v := range pf.Clients {
		if strings.EqualFold(strings.TrimSpace(k), ruleName) {
			return mergePolicy(p, v)
		}
	}

	return p
}

func mergePolicy(def Policy, over Policy) Policy {
	if strings.TrimSpace(over.Client) != "" {
		def.Client = over.Client
	}
	if strings.TrimSpace(over.Urgency) != "" {
		def.Urgency = over.Urgency
	}
	if strings.TrimSpace(over.Impact) != "" {
		def.Impact = over.Impact
	}
	if strings.TrimSpace(over.Priority) != "" {
		def.Priority = over.Priority
	}

	// autoclose do cliente manda (se existir bloco do cliente)
	def.AutoClose = over.AutoClose

	// topdesk: só sobrescreve se cliente tiver valor
	if strings.TrimSpace(over.TopDesk.Contract) != "" {
		def.TopDesk.Contract = over.TopDesk.Contract
	}
	if strings.TrimSpace(over.TopDesk.Operator) != "" {
		def.TopDesk.Operator = over.TopDesk.Operator
	}
	if strings.TrimSpace(over.TopDesk.OperGroup) != "" {
		def.TopDesk.OperGroup = over.TopDesk.OperGroup
	}
	if strings.TrimSpace(over.TopDesk.MainCaller) != "" {
		def.TopDesk.MainCaller = over.TopDesk.MainCaller
	}
	if strings.TrimSpace(over.TopDesk.SecundaryCaller) != "" {
		def.TopDesk.SecundaryCaller = over.TopDesk.SecundaryCaller
	}
	if strings.TrimSpace(over.TopDesk.Sla) != "" {
		def.TopDesk.Sla = over.TopDesk.Sla
	}
	if strings.TrimSpace(over.TopDesk.Category) != "" {
		def.TopDesk.Category = over.TopDesk.Category
	}
	if strings.TrimSpace(over.TopDesk.SubCategory) != "" {
		def.TopDesk.SubCategory = over.TopDesk.SubCategory
	}
	if strings.TrimSpace(over.TopDesk.CallType) != "" {
		def.TopDesk.CallType = over.TopDesk.CallType
	}
	def.TopDesk.SendMoreInfo = over.TopDesk.SendMoreInfo
	if over.TopDesk.MoreInfoText != "" {
		def.TopDesk.MoreInfoText = over.TopDesk.MoreInfoText
	}
	def.TopDesk.AdicionalCresol = over.TopDesk.AdicionalCresol
	def.TopDesk.SendEmail = over.TopDesk.SendEmail
	if over.TopDesk.EmailTo != "" {
		def.TopDesk.EmailTo = over.TopDesk.EmailTo
	}
	if over.TopDesk.EmailCc != "" {
		def.TopDesk.EmailCc = over.TopDesk.EmailCc
	}

	return def
}
