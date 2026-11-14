// Package flowlog guarda o que os fluxos fizeram:
//
//   - histórico (JSON lines): uma linha por execução de fluxo (ativa ou
//     shadow), com evento, host, rule, caminho, campos definidos e erro —
//     lido pela página Fluxos → Histórico (GET /flows/history).
//   - estado por evento: quantas vezes o alerta já passou pelo fluxo
//     (campo event.resends) e quais blocos "atualizar chamado" já rodaram
//     (pra rodar uma vez só por evento). Entradas com mais de 7 dias somem.
//
// Os dois são gravados pelo godesk serve e pelo modo one-shot, com lock de
// arquivo. Falha aqui nunca impede o alerta (o app só registra no log).
package flowlog

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"godesk/internal/flow"
)

// Caminhos padrão: config.DefaultFlowsHistoryFile / DefaultFlowsStateFile.
const (
	// histórico: ao passar de maxHistoryBytes, reescreve mantendo as
	// últimas keepHistory linhas
	maxHistoryBytes = 8 << 20
	keepHistory     = 5000

	stateTTL = 7 * 24 * time.Hour
)

// Entry é uma execução de fluxo.
type Entry struct {
	Time    time.Time         `json:"time"`
	EventID string            `json:"event_id"`
	Host    string            `json:"host"`
	Trigger string            `json:"trigger"`
	Rule    string            `json:"rule"`
	Flow    string            `json:"flow"`
	Mode    string            `json:"mode"` // on | shadow
	Kind    string            `json:"kind"` // problem | recovery
	Ticket  string            `json:"ticket,omitempty"`
	Action  string            `json:"action"`
	Set     []flow.Assignment `json:"set,omitempty"`
	Updates []flow.Update     `json:"updates,omitempty"`
	HTTP    []flow.HTTPCall   `json:"http,omitempty"`
	Trace   []flow.Step       `json:"trace,omitempty"`
	Path    string            `json:"path"`
	Error   string            `json:"error,omitempty"`
	Notes   []string          `json:"notes,omitempty"` // o que o app fez/pulou (ex: update já aplicado)
}

func withLock(path string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lf, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o664)
	if err != nil {
		return err
	}
	defer lf.Close()
	if err := lockFile(lf); err != nil {
		return err
	}
	defer unlockFile(lf)
	return fn()
}

// Append grava uma execução no histórico.
func Append(path string, e Entry) error {
	if path == "" {
		return nil
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return withLock(path, func() error {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o664)
		if err != nil {
			return err
		}
		_, werr := f.Write(append(b, '\n'))
		info, serr := f.Stat()
		f.Close()
		if werr != nil {
			return werr
		}
		if serr == nil && info.Size() > maxHistoryBytes {
			return trim(path)
		}
		return nil
	})
}

func trim(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := bytes.Split(bytes.TrimRight(data, "\n"), []byte("\n"))
	if len(lines) > keepHistory {
		lines = lines[len(lines)-keepHistory:]
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(bytes.Join(lines, []byte("\n")), '\n'), 0o664); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Filter do histórico (campos vazios = sem filtro; texto sem diferenciar
// maiúsculas, "contém").
type Filter struct {
	Flow    string
	Rule    string
	EventID string
	Text    string // procura em host, trigger, rule, fluxo e caminho
	Limit   int
}

// List devolve as execuções mais recentes primeiro.
func List(path string, flt Filter) ([]Entry, error) {
	if flt.Limit <= 0 || flt.Limit > 1000 {
		flt.Limit = 200
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var all []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4<<20)
	for sc.Scan() {
		var e Entry
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		if !match(e, flt) {
			continue
		}
		all = append(all, e)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	out := make([]Entry, 0, flt.Limit)
	for i := len(all) - 1; i >= 0 && len(out) < flt.Limit; i-- {
		out = append(out, all[i])
	}
	return out, nil
}

func match(e Entry, flt Filter) bool {
	has := func(s, sub string) bool {
		return strings.Contains(strings.ToLower(s), strings.ToLower(strings.TrimSpace(sub)))
	}
	if flt.Flow != "" && !strings.EqualFold(e.Flow, strings.TrimSpace(flt.Flow)) {
		return false
	}
	if flt.Rule != "" && !strings.EqualFold(e.Rule, strings.TrimSpace(flt.Rule)) {
		return false
	}
	if flt.EventID != "" && e.EventID != strings.TrimSpace(flt.EventID) {
		return false
	}
	if flt.Text != "" {
		blob := strings.Join([]string{e.Host, e.Trigger, e.Rule, e.Flow, e.Path, e.Ticket, e.EventID}, " ")
		if !has(blob, flt.Text) {
			return false
		}
	}
	return true
}

// ---------- estado por evento ----------

type eventState struct {
	First time.Time `json:"first"`
	Last  time.Time `json:"last"`
	Runs  int       `json:"runs"`
	Done  []string  `json:"done,omitempty"` // blocos update já aplicados
}

type stateFile struct {
	Events map[string]*eventState `json:"events"`
}

func loadState(path string) stateFile {
	st := stateFile{Events: map[string]*eventState{}}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &st)
		if st.Events == nil {
			st.Events = map[string]*eventState{}
		}
	}
	return st
}

func saveState(path string, st stateFile, now time.Time) error {
	for k, e := range st.Events {
		if now.Sub(e.Last) > stateTTL {
			delete(st.Events, k)
		}
	}
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o664); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Touch registra mais uma passagem do evento pelo fluxo e devolve quantas
// houve ANTES (0 = primeira vez) e os blocos update já aplicados.
func Touch(path, eventID string, now time.Time) (prevRuns int, done []string, err error) {
	if path == "" || strings.TrimSpace(eventID) == "" {
		return 0, nil, nil
	}
	err = withLock(path, func() error {
		st := loadState(path)
		e := st.Events[eventID]
		if e == nil {
			e = &eventState{First: now}
			st.Events[eventID] = e
		}
		prevRuns = e.Runs
		done = append([]string(nil), e.Done...)
		e.Runs++
		e.Last = now
		return saveState(path, st, now)
	})
	return prevRuns, done, err
}

// MarkDone registra que o bloco update rodou pra esse evento.
func MarkDone(path, eventID, node string, now time.Time) error {
	if path == "" || strings.TrimSpace(eventID) == "" {
		return nil
	}
	return withLock(path, func() error {
		st := loadState(path)
		e := st.Events[eventID]
		if e == nil {
			e = &eventState{First: now}
			st.Events[eventID] = e
		}
		for _, d := range e.Done {
			if d == node {
				return nil
			}
		}
		e.Done = append(e.Done, node)
		e.Last = now
		return saveState(path, st, now)
	})
}
