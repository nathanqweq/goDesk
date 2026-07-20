package config

import "testing"

func TestParsePoliciesOldFormatUnchanged(t *testing.T) {
	yamlText := `
default:
  urgency: "Baixa"
  impact: "Sem impacto"
  topdesk:
    contract: "DEFAULT"

clients:
  MINHA-REGRA:
    client: "Cliente X"
    urgency: "Alta"
    autoclose: true
    topdesk:
      contract: "CONTRATO-X"
      operator: "OP-X"
      sla: "SLA-X"
`
	pf, err := ParsePolicies([]byte(yamlText))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	pol := ResolvePolicy(pf, "MINHA-REGRA")
	if pol.Urgency != "Alta" {
		t.Fatalf("esperava urgency=Alta, veio %q", pol.Urgency)
	}
	if pol.TopDesk.Contract != "CONTRATO-X" {
		t.Fatalf("esperava contract=CONTRATO-X, veio %q", pol.TopDesk.Contract)
	}
	if !pol.AutoClose {
		t.Fatal("esperava autoclose=true")
	}
}

func TestParsePoliciesNewFormatInheritsFromClient(t *testing.T) {
	yamlText := `
default:
  urgency: "Baixa"
  impact: "Sem impacto"

clients:
  HELPDESK:
    topdesk:
      contract: "CONTRATO-HELPDESK"
      operator: "OP-HELPDESK"
      oper_group: "GRP-HELPDESK"
      sla: "SLA-HELPDESK"
      send_more_info: true
      adicional_cresol: false
      send_email: false

rules:
  HELPDESK-ICMP_VIVO-SPO-WAN2:
    client: HELPDESK
    urgency: "Alta"
    impact: "Servico"
    priority: "2-Alta"
    autoclose: true
    topdesk:
      more_info_text: "Circuito SPO"
  HELPDESK-ICMP_VIVO-BSB-WAN2:
    client: HELPDESK
    urgency: "Alta"
    impact: "Servico"
    priority: "2-Alta"
    autoclose: true
    topdesk:
      more_info_text: "Circuito BSB"
`
	pf, err := ParsePolicies([]byte(yamlText))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	spo := ResolvePolicy(pf, "HELPDESK-ICMP_VIVO-SPO-WAN2")
	bsb := ResolvePolicy(pf, "HELPDESK-ICMP_VIVO-BSB-WAN2")

	for name, pol := range map[string]Policy{"SPO": spo, "BSB": bsb} {
		if pol.TopDesk.Contract != "CONTRATO-HELPDESK" {
			t.Fatalf("[%s] esperava contract herdado do cliente, veio %q", name, pol.TopDesk.Contract)
		}
		if pol.TopDesk.Operator != "OP-HELPDESK" {
			t.Fatalf("[%s] esperava operator herdado do cliente, veio %q", name, pol.TopDesk.Operator)
		}
		if pol.TopDesk.Sla != "SLA-HELPDESK" {
			t.Fatalf("[%s] esperava sla herdado do cliente, veio %q", name, pol.TopDesk.Sla)
		}
		if !pol.TopDesk.SendMoreInfo {
			t.Fatalf("[%s] esperava send_more_info herdado do cliente (true)", name)
		}
		if !pol.AutoClose {
			t.Fatalf("[%s] esperava autoclose=true (da regra)", name)
		}
	}

	if spo.TopDesk.MoreInfoText != "Circuito SPO" {
		t.Fatalf("esperava more_info_text específico da regra SPO, veio %q", spo.TopDesk.MoreInfoText)
	}
	if bsb.TopDesk.MoreInfoText != "Circuito BSB" {
		t.Fatalf("esperava more_info_text específico da regra BSB, veio %q", bsb.TopDesk.MoreInfoText)
	}
}

func TestParsePoliciesRuleWithUnknownClientFallsBackToDefault(t *testing.T) {
	yamlText := `
default:
  topdesk:
    contract: "CONTRATO-DEFAULT"

clients:
  HELPDESK:
    topdesk:
      contract: "CONTRATO-HELPDESK"

rules:
  REGRA-SEM-CLIENTE-VALIDO:
    client: NAO-EXISTE
    urgency: "Alta"
`
	pf, err := ParsePolicies([]byte(yamlText))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	pol := ResolvePolicy(pf, "REGRA-SEM-CLIENTE-VALIDO")
	if pol.TopDesk.Contract != "CONTRATO-DEFAULT" {
		t.Fatalf("esperava cair no contract do default, veio %q", pol.TopDesk.Contract)
	}
}

func TestParsePoliciesRuleWithoutClientControlsOwnBooleans(t *testing.T) {
	yamlText := `
default:
  topdesk:
    send_more_info: false

clients:
  HELPDESK:
    topdesk:
      send_more_info: false

rules:
  REGRA-STANDALONE:
    topdesk:
      contract: "CONTRATO-PROPRIO"
      send_more_info: true
`
	pf, err := ParsePolicies([]byte(yamlText))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	pol := ResolvePolicy(pf, "REGRA-STANDALONE")
	if pol.TopDesk.Contract != "CONTRATO-PROPRIO" {
		t.Fatalf("esperava contract próprio da regra, veio %q", pol.TopDesk.Contract)
	}
	if !pol.TopDesk.SendMoreInfo {
		t.Fatal("regra sem cliente deveria controlar seus próprios booleanos (send_more_info=true)")
	}
}

func TestParsePoliciesRuleTextOverrideWinsOverClient(t *testing.T) {
	yamlText := `
default: {}

clients:
  HELPDESK:
    topdesk:
      category: "7 - Monitoramento"

rules:
  REGRA-CATEGORIA-CUSTOM:
    client: HELPDESK
    topdesk:
      category: "9 - Categoria Especial"
`
	pf, err := ParsePolicies([]byte(yamlText))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	pol := ResolvePolicy(pf, "REGRA-CATEGORIA-CUSTOM")
	if pol.TopDesk.Category != "9 - Categoria Especial" {
		t.Fatalf("esperava override da regra vencer, veio %q", pol.TopDesk.Category)
	}
}

func TestParsePoliciesOncePerDayInheritsFromClient(t *testing.T) {
	yamlText := `
default: {}

clients:
  HELPDESK:
    topdesk:
      send_email: true
      once_per_day: true

rules:
  REGRA-UM-POR-DIA:
    client: HELPDESK
`
	pf, err := ParsePolicies([]byte(yamlText))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	pol := ResolvePolicy(pf, "REGRA-UM-POR-DIA")
	if !pol.TopDesk.OncePerDay {
		t.Fatal("esperava once_per_day herdado do cliente (true)")
	}
}

func TestParsePoliciesRuleWithoutClientControlsOwnOncePerDay(t *testing.T) {
	yamlText := `
default:
  topdesk:
    once_per_day: false

clients:
  HELPDESK:
    topdesk:
      once_per_day: false

rules:
  REGRA-STANDALONE:
    topdesk:
      contract: "CONTRATO-PROPRIO"
      once_per_day: true
`
	pf, err := ParsePolicies([]byte(yamlText))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	pol := ResolvePolicy(pf, "REGRA-STANDALONE")
	if !pol.TopDesk.OncePerDay {
		t.Fatal("regra sem cliente deveria controlar seu próprio once_per_day (true)")
	}
}
