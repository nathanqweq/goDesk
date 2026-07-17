package app

import (
	"log"

	"godesk/internal/metrics"
)

// recordMetric aplica fn ao snapshot de métricas persistido em path,
// só logando um aviso se a gravação falhar — nunca deve alterar o
// resultado do processamento do alerta em si.
func recordMetric(path string, fn func(*metrics.Snapshot)) {
	if _, err := metrics.Record(path, fn); err != nil {
		log.Printf("[metrics] WARN: falha ao gravar métricas (%s): %v\n", path, err)
	}
}
