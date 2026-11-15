//go:build !unix

package metrics

import (
	"os"
	"sync"
)

// Fora de unix (desenvolvimento/testes no Windows) não há flock: um mutex
// cobre a concorrência dentro do processo — o goDesk só roda em produção
// no Linux.
var mu sync.Mutex

func lockFile(f *os.File) error { mu.Lock(); return nil }

func unlockFile(f *os.File) { mu.Unlock() }
