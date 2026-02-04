package rawdata

import (
	"encoding/json"
	"errors"
	"fmt"
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

	// validações mínimas
	if p.EventID == "" {
		return Payload{}, errors.New("RAWDATA inválido: event_id vazio")
	}
	if p.Trigger == "" {
		return Payload{}, errors.New("RAWDATA inválido: trigger vazio")
	}
	if p.Cliente == "" {
		// não é obrigatório, mas você usa pra policy
		return Payload{}, errors.New("RAWDATA inválido: cliente vazio (EVENT.TAGS.Cliente)")
	}
	if p.EventValue != "0" && p.EventValue != "1" {
		return Payload{}, fmt.Errorf("RAWDATA inválido: event_value deve ser '0' ou '1' (veio %q)", p.EventValue)
	}

	return p, nil
}

func EventKind(p Payload) string {
	if p.EventValue == "1" {
		return "ProblemStart"
	}
	return "ProblemRecovery"
}
