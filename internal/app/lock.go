package app

import "sync"

// ticketLocks garante que a checagem TicketExists e a criação/atualização
// que vem em seguida, para um mesmo ticketName, nunca rodem concorrentemente
// — evita criar chamado duplicado quando dois alertas do mesmo trigger
// chegam quase juntos (possível desde que godesk serve processa
// requisições em goroutines paralelas). ticketNames diferentes não se
// bloqueiam entre si.
var ticketLocks sync.Map // map[string]*sync.Mutex

// lockTicket trava o processamento do ticketName informado e devolve a
// função de destravamento (chame com defer logo em seguida).
func lockTicket(ticketName string) func() {
	v, _ := ticketLocks.LoadOrStore(ticketName, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}
