package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"godesk/internal/config"
	"godesk/internal/topdesk"
)

type validateRequest struct {
	Operator  string `json:"operator"`
	OperGroup string `json:"oper_group"`
}

type fieldResult struct {
	Valid bool   `json:"valid"`
	Error string `json:"error,omitempty"`
}

// handleValidateClient confere se operator/oper_group existem no TopDesk —
// só valida os campos que vierem preenchidos na requisição. Usado pelo
// botão "Testar" do card de Cliente no módulo Zabbix (via godesk-client,
// já que só o servidor tem as credenciais do TopDesk).
func handleValidateClient(opts config.ServiceConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, opts.Token) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "falha ao ler corpo: " + err.Error()})
			return
		}

		var req validateRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "json inválido: " + err.Error()})
			return
		}

		client := topdesk.Client{
			BaseURL: opts.TopdeskDomain,
			User:    opts.TopdeskUser,
			Pass:    opts.TopdeskPass,
			HTTP:    &http.Client{Timeout: 15 * time.Second},
		}

		result := map[string]fieldResult{}
		if v := strings.TrimSpace(req.Operator); v != "" {
			result["operator"] = checkTopdeskID(client.OperatorExists, v)
		}
		if v := strings.TrimSpace(req.OperGroup); v != "" {
			result["oper_group"] = checkTopdeskID(client.OperatorGroupExists, v)
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func checkTopdeskID(exists func(string) (bool, error), id string) fieldResult {
	ok, err := exists(id)
	if err != nil {
		return fieldResult{Valid: false, Error: err.Error()}
	}
	if !ok {
		return fieldResult{Valid: false, Error: "não encontrado no TopDesk"}
	}
	return fieldResult{Valid: true}
}
