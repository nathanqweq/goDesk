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
		return Payload{}, fmt.Errorf("RAWDATA não é JSON válido: %w", err)
	}

	Normalize(&p)

	// --- normaliza event_value para "0"/"1" (aceita " 1 ", "0", etc.)
	p.EventValue = strings.TrimSpace(p.EventValue)
	p.EventValue = strings.Trim(p.EventValue, `"'`)

	// validações mínimas
	if strings.TrimSpace(p.EventID) == "" {
		return Payload{}, errors.New("RAWDATA inválido: event_id vazio")
	}
	if strings.TrimSpace(p.Trigger) == "" {
		return Payload{}, errors.New("RAWDATA inválido: trigger vazio")
	}

	// Cliente NÃO obrigatório: se não vier, cai no default do YAML
	p.Cliente = strings.TrimSpace(p.Cliente)

	// valida event_value:
	// - aceita "0"/"1"
	// - se vier como "0.0" ou "1.0" ou "0 " etc, tenta converter
	if p.EventValue != "0" && p.EventValue != "1" {
		// tenta converter string numérica → int
		if n, err := strconv.Atoi(p.EventValue); err == nil && (n == 0 || n == 1) {
			p.EventValue = strconv.Itoa(n)
		} else {
			return Payload{}, fmt.Errorf("RAWDATA inválido: event_value deve ser '0' ou '1' (veio %q)", p.EventValue)
		}
	}

	return p, nil
}

func EventKind(p Payload) string {
	if p.EventValue == "1" {
		return "ProblemStart"
	}
	return "ProblemRecovery"
}
