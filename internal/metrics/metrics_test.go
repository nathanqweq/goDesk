package metrics

import (
	"path/filepath"
	"sync"
	"testing"
)

func TestReadNonExistentFileReturnsZeroSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")

	snap, err := Read(path)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if snap.AlertsProcessed != 0 || !snap.StartedAt.IsZero() {
		t.Fatalf("esperava snapshot zerado, veio %+v", snap)
	}
}

func TestRecordCreatesFileAndSetsStartedAt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")

	snap, err := Record(path, func(s *Snapshot) {
		s.AlertsProcessed++
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if snap.AlertsProcessed != 1 {
		t.Fatalf("esperava AlertsProcessed=1, veio %d", snap.AlertsProcessed)
	}
	if snap.StartedAt.IsZero() {
		t.Fatal("esperava StartedAt preenchido na primeira gravação")
	}
	if snap.UpdatedAt.IsZero() {
		t.Fatal("esperava UpdatedAt preenchido")
	}
}

func TestRecordPersistsAcrossCalls(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")

	first, err := Record(path, func(s *Snapshot) { s.TicketsCreated++ })
	if err != nil {
		t.Fatalf("erro na primeira gravação: %v", err)
	}

	second, err := Record(path, func(s *Snapshot) { s.TicketsCreated++ })
	if err != nil {
		t.Fatalf("erro na segunda gravação: %v", err)
	}

	if second.TicketsCreated != 2 {
		t.Fatalf("esperava TicketsCreated=2, veio %d", second.TicketsCreated)
	}
	if !second.StartedAt.Equal(first.StartedAt) {
		t.Fatalf("StartedAt não deveria mudar entre gravações: %v != %v", second.StartedAt, first.StartedAt)
	}

	fromDisk, err := Read(path)
	if err != nil {
		t.Fatalf("erro ao ler: %v", err)
	}
	if fromDisk.TicketsCreated != 2 {
		t.Fatalf("esperava TicketsCreated=2 lendo do disco, veio %d", fromDisk.TicketsCreated)
	}
}

func TestRecordConcurrentIncrementsAreConsistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")

	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := Record(path, func(s *Snapshot) { s.AlertsProcessed++ }); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("erro durante gravação concorrente: %v", err)
	}

	final, err := Read(path)
	if err != nil {
		t.Fatalf("erro ao ler resultado final: %v", err)
	}
	if final.AlertsProcessed != n {
		t.Fatalf("esperava AlertsProcessed=%d (sem perda de incremento por corrida), veio %d", n, final.AlertsProcessed)
	}
}
