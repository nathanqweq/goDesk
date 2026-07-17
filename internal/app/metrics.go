package app

import "godesk/internal/metrics"

// recordMetric aplica fn ao snapshot de métricas persistido em path,
// só logando um aviso se a gravação falhar — nunca deve alterar o
// resultado do processamento do alerta em si.
func recordMetric(path string, fn func(*metrics.Snapshot)) {
	metrics.RecordAndLog(path, fn)
}
