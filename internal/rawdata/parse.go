package rawdata

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func Parse(raw string) (Payload, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Payload{}, errors.New("RAWDATA vazio")
	}

	var p Payload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return Payload{}, fmt.Errorf("RAWDATA não é JSON válido: %w%s", err, mediaTypeHint(raw))
	}

	Normalize(&p)

	// normaliza event_value para "0"/"1"
	p.EventValue = strings.TrimSpace(p.EventValue)
	p.EventValue = strings.Trim(p.EventValue, `"'`)

	// validações mínimas
	if strings.TrimSpace(p.EventID) == "" {
		return Payload{}, errors.New("RAWDATA inválido: event_id vazio")
	}
	if strings.TrimSpace(p.Trigger) == "" {
		return Payload{}, errors.New("RAWDATA inválido: trigger vazio")
	}
	if strings.TrimSpace(p.RuleName) == "" {
		return Payload{}, errors.New("RAWDATA inválido: rule_name vazio (EVENT.TAGS.RuleName)")
	}

	// aceita "0"/"1" (ou string numérica)
	if p.EventValue != "0" && p.EventValue != "1" {
		if n, err := strconv.Atoi(p.EventValue); err == nil && (n == 0 || n == 1) {
			p.EventValue = strconv.Itoa(n)
		} else {
			return Payload{}, fmt.Errorf("RAWDATA inválido: event_value deve ser '0' ou '1' (veio %q)", p.EventValue)
		}
	}

	// cliente agora é display (não obrigatório)
	p.Cliente = strings.TrimSpace(p.Cliente)

	return p, nil
}

// mediaTypeHint detecta o sintoma mais comum de Media Type do Zabbix
// desatualizado: desde que godesk passou a esperar só "TICKET_NAME RAWDATA"
// (2 parâmetros), um Media Type ainda configurado com os 7 antigos (DOMAIN
// USER PASS TICKET_NAME RAWDATA ZABBIX_URL ZABBIX_KEY) faz o RAWDATA
// receber, na real, o valor do antigo parâmetro USER — um texto qualquer
// que nunca começa com "{". Isso não cobre todo erro de JSON malformado
// (ex: aspas curvas), só esse caso específico e bem comum.
func mediaTypeHint(raw string) string {
	if strings.HasPrefix(raw, "{") {
		return ""
	}
	return " — RAWDATA não começa com '{': isso costuma indicar que o Media Type do Zabbix ainda está configurado com os 7 parâmetros antigos (DOMAIN USER PASS TICKET_NAME RAWDATA ZABBIX_URL ZABBIX_KEY) em vez dos 2 atuais (TICKET_NAME RAWDATA) — confira em Administration → Alerts → Media types → Parameters"
}

func EventKind(p Payload) string {
	if p.EventValue == "1" {
		return "ProblemStart"
	}
	return "ProblemRecovery"
}
