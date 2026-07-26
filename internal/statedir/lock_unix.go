//go:build !windows

package statedir

import (
	"os"

	"golang.org/x/sys/unix"
)

// lockFile takes a non-blocking exclusive flock. The lock lives on the open
// file description, so a second descriptor for the same file is denied even
// within one process.
func lockFile(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
}

func unlockFile(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_UN)
}
