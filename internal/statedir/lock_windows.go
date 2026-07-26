//go:build windows

package statedir

import (
	"os"

	"golang.org/x/sys/windows"
)

// lockFile takes an exclusive byte-range lock that fails immediately when the
// range is already held. One byte is enough: the range is a token, not data.
func lockFile(f *os.File) error {
	return windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, new(windows.Overlapped),
	)
}

func unlockFile(f *os.File) error {
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, new(windows.Overlapped))
}
