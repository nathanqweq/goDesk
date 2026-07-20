package app

import (
	"sync"
	"time"
)

// OncePerDayEntry representa um alerta+host que já teve e-mail enviado hoje
// (feature "um por dia" — TopDeskDefaults.OncePerDay, config.ServiceConfig
// nao participa: o estado vive só na memória do processo godesk serve).
type OncePerDayEntry struct {
	Client     string    `json:"client,omitempty"`
	Alert      string    `json:"alert"`
	Host       string    `json:"host,omitempty"`
	HostID     string    `json:"host_id"`
	Day        string    `json:"day"`
	RecordedAt time.Time `json:"recorded_at"`
}

var (
	oncePerDayMu   sync.Mutex
	oncePerDaySeen = map[string]OncePerDayEntry{}
)

func oncePerDayKey(alert, hostID string) string {
	return alert + "\x00" + hostID
}

func oncePerDayToday() string {
	return time.Now().Format("2006-01-02")
}

// oncePerDaySeenToday confere se alert+hostID já teve e-mail enviado hoje —
// entradas de dias anteriores não contam (a lista "esquece" sozinha na
// virada de 00:00, sem depender de um timer preciso).
func oncePerDaySeenToday(alert, hostID string) bool {
	oncePerDayMu.Lock()
	defer oncePerDayMu.Unlock()

	e, ok := oncePerDaySeen[oncePerDayKey(alert, hostID)]
	return ok && e.Day == oncePerDayToday()
}

// oncePerDayRecord marca alert+hostID como enviado hoje — só deve ser
// chamado depois de um envio de e-mail bem-sucedido.
func oncePerDayRecord(client, alert, host, hostID string) {
	oncePerDayMu.Lock()
	defer oncePerDayMu.Unlock()

	oncePerDaySeen[oncePerDayKey(alert, hostID)] = OncePerDayEntry{
		Client:     client,
		Alert:      alert,
		Host:       host,
		HostID:     hostID,
		Day:        oncePerDayToday(),
		RecordedAt: time.Now(),
	}
}

// OncePerDayPurge remove entradas de dias anteriores — só higiene de
// memória, chamado periodicamente por uma goroutine em internal/server;
// não afeta a correção de oncePerDaySeenToday, que já ignora dias antigos.
func OncePerDayPurge() {
	oncePerDayMu.Lock()
	defer oncePerDayMu.Unlock()

	today := oncePerDayToday()
	for k, e := range oncePerDaySeen {
		if e.Day != today {
			delete(oncePerDaySeen, k)
		}
	}
}

// OncePerDayList devolve as entradas registradas hoje, para acompanhamento
// (endpoint GET /once-per-day, comando `godesk --once-per-day`).
func OncePerDayList() []OncePerDayEntry {
	oncePerDayMu.Lock()
	defer oncePerDayMu.Unlock()

	today := oncePerDayToday()
	out := make([]OncePerDayEntry, 0, len(oncePerDaySeen))
	for _, e := range oncePerDaySeen {
		if e.Day == today {
			out = append(out, e)
		}
	}
	return out
}
