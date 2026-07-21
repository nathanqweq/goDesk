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
	OncePerDay      bool   `yaml:"once_per_day"`
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

// ClientPolicy é um cliente nomeado no formato novo do YAML (clientes +
// rules) — só carrega o TopDesk compartilhado por todas as rules que o
// referenciam.
type ClientPolicy struct {
	TopDesk TopDeskDefaults `yaml:"topdesk"`
}

// RulePolicy é uma regra no formato novo: referencia um ClientPolicy (pelo
// nome, chave em policiesFileV2.Clients) e só carrega o que de fato varia
// por regra — urgency/impact/priority/autoclose e overrides de texto do
// TopDesk (o mais comum sendo more_info_text).
type RulePolicy struct {
	Client    string          `yaml:"client"`
	Priority  string          `yaml:"priority"`
	Urgency   string          `yaml:"urgency"`
	Impact    string          `yaml:"impact"`
	AutoClose bool            `yaml:"autoclose"`
	TopDesk   TopDeskDefaults `yaml:"topdesk"`
}

// policiesFileV2 é o formato novo do YAML — só usado internamente durante
// o parse, nunca exposto fora deste pacote. Detectado pela presença da
// chave "rules" (ver ParsePolicies).
type policiesFileV2 struct {
	Default Policy                  `yaml:"default"`
	Clients map[string]ClientPolicy `yaml:"clients"`
	Rules   map[string]RulePolicy   `yaml:"rules"`
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
//
// Aceita dois formatos: o antigo — "clients: {ruleName: {...topdesk
// completo...}}" — e o novo — "clients: {clientName: {topdesk}}" +
// "rules: {ruleName: {client: clientName, overrides}}" — que elimina a
// duplicação do bloco topdesk quando várias regras pertencem ao mesmo
// cliente. O formato novo é detectado pela presença da chave "rules" não
// vazia; sem ela, o comportamento é idêntico ao de sempre.
func ParsePolicies(data []byte) (PoliciesFile, error) {
	var probe struct {
		Rules map[string]RulePolicy `yaml:"rules"`
	}
	if err := yaml.Unmarshal(data, &probe); err != nil {
		return PoliciesFile{}, err
	}

	var pf PoliciesFile
	if len(probe.Rules) > 0 {
		var v2 policiesFileV2
		if err := yaml.Unmarshal(data, &v2); err != nil {
			return PoliciesFile{}, err
		}
		pf = flattenRules(v2)
	} else if err := yaml.Unmarshal(data, &pf); err != nil {
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

// flattenRules resolve o formato "clientes + rules" para o
// map[string]Policy achatado que ResolvePolicy já sabe consumir — cada
// regra herda o topdesk do cliente referenciado (incluindo os 4 campos
// booleanos send_more_info/adicional_cresol/send_email/once_per_day) e só
// pode sobrescrever o que de fato varia por regra: urgency/impact/priority/
// autoclose e os campos de texto do topdesk. Quando a regra referencia um
// cliente válido, os 4 booleanos do topdesk vêm sempre do cliente — bool
// não distingue "não informado" de "false", então permitir override por
// regra apagaria silenciosamente o que o cliente configurou assim que a
// regra não preenchesse esses campos. Regras sem cliente (referência
// vazia ou inexistente) continuam funcionando como o formato antigo: o
// topdesk da própria regra é aplicado por completo, booleanos inclusos.
func flattenRules(v2 policiesFileV2) PoliciesFile {
	out := make(map[string]Policy, len(v2.Rules))

	for ruleName, rule := range v2.Rules {
		p := v2.Default

		cp, hasClient := v2.Clients[rule.Client]
		hasClient = hasClient && rule.Client != ""

		if hasClient {
			p.TopDesk = mergeTopDesk(p.TopDesk, cp.TopDesk)
			p.Client = rule.Client
			p.TopDesk = mergeTopDeskTextOverrides(p.TopDesk, rule.TopDesk)
		} else {
			p.TopDesk = mergeTopDesk(p.TopDesk, rule.TopDesk)
		}

		if strings.TrimSpace(rule.Urgency) != "" {
			p.Urgency = rule.Urgency
		}
		if strings.TrimSpace(rule.Impact) != "" {
			p.Impact = rule.Impact
		}
		if strings.TrimSpace(rule.Priority) != "" {
			p.Priority = rule.Priority
		}
		p.AutoClose = rule.AutoClose

		out[ruleName] = p
	}

	return PoliciesFile{Default: v2.Default, Clients: out}
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

// TestWritable confere se dá pra gravar em path sem alterar seu conteúdo:
// cria e remove um arquivo temporário no mesmo diretório (mesma operação
// que SaveFile precisa pra criar+renomear), nunca tocando no arquivo real.
// Usado pelo endpoint POST /config/test (godesk-client test-config) pra
// testar send+save do config sem sobrescrever o arquivo de produção.
func TestWritable(path string) error {
	tmp := path + ".writetest." + time.Now().Format("20060102-150405.000000000")

	if err := os.WriteFile(tmp, []byte("godesk write test"), 0644); err != nil {
		return err
	}

	return os.Remove(tmp)
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

	def.TopDesk = mergeTopDesk(def.TopDesk, over.TopDesk)

	return def
}

// mergeTopDesk aplica over sobre base como uma política completa: campos
// de texto só sobrescrevem se vierem preenchidos; os 4 booleanos
// (send_more_info, adicional_cresol, send_email, once_per_day) sempre vêm
// de over. Use isso quando over representa uma política inteira (cliente
// do YAML antigo, ou cliente nomeado do YAML novo) — para overrides
// parciais de regra, veja mergeTopDeskTextOverrides.
func mergeTopDesk(base, over TopDeskDefaults) TopDeskDefaults {
	base = mergeTopDeskTextOverrides(base, over)
	base.SendMoreInfo = over.SendMoreInfo
	base.AdicionalCresol = over.AdicionalCresol
	base.SendEmail = over.SendEmail
	base.OncePerDay = over.OncePerDay
	return base
}

// mergeTopDeskTextOverrides aplica só os campos de texto de over sobre
// base (inclui more_info_text), deliberadamente sem tocar nos 4 booleanos
// — usado pelo override de uma regra sobre o que já foi resolvido do
// cliente/default.
func mergeTopDeskTextOverrides(base, over TopDeskDefaults) TopDeskDefaults {
	if strings.TrimSpace(over.Contract) != "" {
		base.Contract = over.Contract
	}
	if strings.TrimSpace(over.Operator) != "" {
		base.Operator = over.Operator
	}
	if strings.TrimSpace(over.OperGroup) != "" {
		base.OperGroup = over.OperGroup
	}
	if strings.TrimSpace(over.MainCaller) != "" {
		base.MainCaller = over.MainCaller
	}
	if strings.TrimSpace(over.SecundaryCaller) != "" {
		base.SecundaryCaller = over.SecundaryCaller
	}
	if strings.TrimSpace(over.Sla) != "" {
		base.Sla = over.Sla
	}
	if strings.TrimSpace(over.Category) != "" {
		base.Category = over.Category
	}
	if strings.TrimSpace(over.SubCategory) != "" {
		base.SubCategory = over.SubCategory
	}
	if strings.TrimSpace(over.CallType) != "" {
		base.CallType = over.CallType
	}
	if over.MoreInfoText != "" {
		base.MoreInfoText = over.MoreInfoText
	}
	if over.EmailTo != "" {
		base.EmailTo = over.EmailTo
	}
	if over.EmailCc != "" {
		base.EmailCc = over.EmailCc
	}
	return base
}
