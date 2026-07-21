package server

import (
	"io"
	"net/http"

	"godesk/internal/config"
)

// handleConfigTest é a versão "dry run" de handleConfig: valida o YAML
// recebido e confere se o servidor consegue gravar no diretório do
// config, mas nunca grava (nem faz backup de) o arquivo real — usado por
// `godesk-client test-config` pra testar o canal client→server sem
// arriscar sobrescrever a config de produção com um payload de teste.
func handleConfigTest(token, configFile string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, token) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "falha ao ler corpo: " + err.Error()})
			return
		}

		if _, err := config.ParsePolicies(body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "yaml inválido: " + err.Error()})
			return
		}

		if err := config.TestWritable(configFile); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "sem permissão de escrita em " + configFile + ": " + err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
