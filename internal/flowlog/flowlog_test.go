package flowlog

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryAppendListFilter(t *testing.T) {
	p := filepath.Join(t.TempDir(), "h.jsonl")
	base := time.Date(2026, 11, 14, 10, 0, 0, 0, time.Local)
	for i := 0; i < 5; i++ {
		rule := "A"
		if i%2 == 1 {
			rule = "B"
		}
		if err := Append(p, Entry{Time: base.Add(time.Duration(i) * time.Minute), EventID: fmt.Sprint(100 + i), Host: fmt.Sprintf("SW_%d", i), Rule: rule, Flow: "infra", Path: "inicio → fim"}); err != nil {
			t.Fatal(err)
		}
	}

	all, err := List(p, Filter{})
	if err != nil || len(all) != 5 || all[0].EventID != "104" {
		t.Fatalf("List = %d itens (err=%v), primeiro=%v; esperado 5, mais recente primeiro", len(all), err, all)
	}
	b, _ := List(p, Filter{Rule: "b"})
	if len(b) != 2 {
		t.Errorf("filtro rule=b: %d, esperado 2", len(b))
	}
	ev, _ := List(p, Filter{EventID: "102"})
	if len(ev) != 1 || ev[0].Host != "SW_2" {
		t.Errorf("filtro evento: %+v", ev)
	}
	txt, _ := List(p, Filter{Text: "sw_3"})
	if len(txt) != 1 {
		t.Errorf("filtro texto: %d", len(txt))
	}
	lim, _ := List(p, Filter{Limit: 2})
	if len(lim) != 2 {
		t.Errorf("limite: %d", len(lim))
	}

	none, err := List(filepath.Join(t.TempDir(), "nao-existe"), Filter{})
	if err != nil || len(none) != 0 {
		t.Errorf("arquivo inexistente = lista vazia, veio %v %v", none, err)
	}
}

func TestHistoryTrim(t *testing.T) {
	p := filepath.Join(t.TempDir(), "h.jsonl")
	// escreve direto um arquivo grande e força o trim
	big := make([]byte, 0, maxHistoryBytes+1024)
	line := []byte(`{"event_id":"x","path":"` + string(make([]byte, 0)) + `aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}` + "\n")
	for len(big) < maxHistoryBytes {
		big = append(big, line...)
	}
	if err := os.WriteFile(p, big, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Append(p, Entry{EventID: "ultimo"}); err != nil {
		t.Fatal(err)
	}
	all, err := List(p, Filter{Limit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if all[0].EventID != "ultimo" {
		t.Errorf("última entrada perdida no trim: %v", all[0])
	}
	info, _ := os.Stat(p)
	if info.Size() > maxHistoryBytes {
		t.Errorf("arquivo não foi reduzido: %d bytes", info.Size())
	}
}

func TestStateTouchAndDone(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.json")
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.Local)

	runs, done, err := Touch(p, "1", now)
	if err != nil || runs != 0 || len(done) != 0 {
		t.Fatalf("primeira passagem: runs=%d done=%v err=%v", runs, done, err)
	}
	if err := MarkDone(p, "1", "escalar", now); err != nil {
		t.Fatal(err)
	}
	_ = MarkDone(p, "1", "escalar", now) // repetido não duplica
	runs, done, _ = Touch(p, "1", now.Add(10*time.Minute))
	if runs != 1 || len(done) != 1 || done[0] != "escalar" {
		t.Errorf("segunda passagem: runs=%d done=%v", runs, done)
	}
	runs, _, _ = Touch(p, "2", now)
	if runs != 0 {
		t.Errorf("outro evento começa do zero: %d", runs)
	}

	// mais de 7 dias sem passar: some
	runs, _, _ = Touch(p, "2", now.Add(8*24*time.Hour))
	if runs != 1 {
		t.Errorf("evento 2 ainda devia existir (acabou de passar): %d", runs)
	}
	runs, _, _ = Touch(p, "1", now.Add(8*24*time.Hour))
	if runs != 0 {
		t.Errorf("evento 1 devia ter expirado: runs=%d", runs)
	}
}
