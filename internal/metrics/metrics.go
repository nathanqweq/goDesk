package metrics

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Snapshot é o estado acumulado de métricas do goDesk, persistido em disco
// para sobreviver a restarts do serviço.
type Snapshot struct {
	StartedAt time.Time `json:"started_at"`
	UpdatedAt time.Time `json:"updated_at"`

	AlertsProcessed int64 `json:"alerts_processed_total"`
	AlertsFailed    int64 `json:"alerts_failed_total"`

	TicketsCreated      int64 `json:"tickets_created_total"`
	TicketsCreateErrors int64 `json:"tickets_create_errors_total"`

	TicketsUpdated      int64 `json:"tickets_updated_total"`
	TicketsUpdateErrors int64 `json:"tickets_update_errors_total"`

	TicketsClosed        int64 `json:"tickets_closed_total"`
	TicketsCloseErrors   int64 `json:"tickets_close_errors_total"`
	TicketsAlreadyClosed int64 `json:"tickets_already_closed_total"`

	ZabbixAckErrors    int64 `json:"zabbix_ack_errors_total"`
	EmailSendErrors    int64 `json:"email_send_errors_total"`
	SendMoreInfoErrors int64 `json:"send_more_info_errors_total"`

	TopdeskHealthChecksTotal        int64     `json:"topdesk_healthchecks_total"`
	TopdeskHealthCheckErrors        int64     `json:"topdesk_healthcheck_errors_total"`
	TopdeskHealthCheckLastOK        bool      `json:"topdesk_healthcheck_last_ok"`
	TopdeskHealthCheckLastAt        time.Time `json:"topdesk_healthcheck_last_at"`
	TopdeskHealthCheckLastLatencyMs int64     `json:"topdesk_healthcheck_last_latency_ms"`
	TopdeskHealthCheckLastError     string    `json:"topdesk_healthcheck_last_error,omitempty"`
}

// Record lê o snapshot atual em path (criando um zerado se o arquivo ainda
// não existir), aplica fn, grava de volta atomicamente e devolve o
// resultado. Usa flock no arquivo para serializar acesso concorrente entre
// goroutines (godesk serve) e entre processos diferentes (invocações
// one-shot), já que múltiplos escritores podem tentar atualizar o mesmo
// arquivo ao mesmo tempo.
func Record(path string, fn func(*Snapshot)) (Snapshot, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return Snapshot{}, err
	}

	lf, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return Snapshot{}, err
	}
	defer lf.Close()

	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		return Snapshot{}, err
	}
	defer syscall.Flock(int(lf.Fd()), syscall.LOCK_UN)

	snap, err := readFile(path)
	if err != nil {
		return Snapshot{}, err
	}
	if snap.StartedAt.IsZero() {
		snap.StartedAt = time.Now().UTC()
	}

	fn(&snap)
	snap.UpdatedAt = time.Now().UTC()

	b, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return Snapshot{}, err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return Snapshot{}, err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return Snapshot{}, err
	}

	return snap, nil
}

// RecordAndLog é um atalho pra Record que só loga (não propaga) erro de
// gravação — usado nos pontos de instrumentação que não devem alterar o
// resultado do fluxo principal (processar alerta, healthcheck) por causa
// de uma falha ao persistir métricas.
func RecordAndLog(path string, fn func(*Snapshot)) {
	if _, err := Record(path, fn); err != nil {
		log.Printf("[metrics] WARN: falha ao gravar métricas (%s): %v\n", path, err)
	}
}

// Read lê o snapshot atual sem lock — seguro porque Record sempre escreve
// atomicamente (tmp + rename), então nunca existe um arquivo parcialmente
// escrito para ler. Se o arquivo ainda não existir, devolve um Snapshot{}
// zerado (estado válido de "nenhum alerta processado ainda"), sem erro.
func Read(path string) (Snapshot, error) {
	return readFile(path)
}

func readFile(path string) (Snapshot, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Snapshot{}, nil
		}
		return Snapshot{}, err
	}

	var snap Snapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		return Snapshot{}, err
	}
	return snap, nil
}
