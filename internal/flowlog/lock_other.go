//go:build !unix

package flowlog

import "os"

// Fora de unix (desenvolvimento/testes no Windows) não há flock: o
// goDesk só roda em produção no Linux.
func lockFile(f *os.File) error { return nil }

func unlockFile(f *os.File) {}
