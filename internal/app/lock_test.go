package app

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLockTicketSerializesSameName(t *testing.T) {
	var inCriticalSection atomic.Bool
	var violated atomic.Bool

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock := lockTicket("same-ticket")
			defer unlock()

			if !inCriticalSection.CompareAndSwap(false, true) {
				violated.Store(true)
				return
			}
			time.Sleep(5 * time.Millisecond)
			inCriticalSection.Store(false)
		}()
	}
	wg.Wait()

	if violated.Load() {
		t.Fatal("duas goroutines entraram na seção crítica do mesmo ticketName ao mesmo tempo")
	}
}

func TestLockTicketAllowsDifferentNamesConcurrently(t *testing.T) {
	var barrier sync.WaitGroup
	barrier.Add(2)

	run := func(name string) {
		unlock := lockTicket(name)
		defer unlock()
		barrier.Done()
		barrier.Wait() // só destrava se as duas goroutines conseguirem entrar ao mesmo tempo
	}

	done := make(chan struct{})
	go func() {
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); run("ticket-a") }()
		go func() { defer wg.Done(); run("ticket-b") }()
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("locks de ticketNames diferentes bloquearam uma a outra (deveriam ser independentes)")
	}
}

func TestLockTicketReusesMutexForSameName(t *testing.T) {
	lockTicket("reuse-me")()

	v1, _ := ticketLocks.Load("reuse-me")

	lockTicket("reuse-me")()

	v2, _ := ticketLocks.Load("reuse-me")

	if v1 != v2 {
		t.Fatal("lockTicket deveria reaproveitar o mesmo mutex para o mesmo ticketName")
	}
}
