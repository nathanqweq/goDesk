//go:build unix

package flowlog

import (
	"os"
	"syscall"
)

// lockFile trava o arquivo de lock (exclusivo) — o modo one-shot
// (alertscript) e o godesk serve podem escrever ao mesmo tempo.
func lockFile(f *os.File) error { return syscall.Flock(int(f.Fd()), syscall.LOCK_EX) }

func unlockFile(f *os.File) { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }
