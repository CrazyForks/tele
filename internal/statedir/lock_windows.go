//go:build windows

package statedir

import (
	"os"

	"golang.org/x/sys/windows"
)

// lockOffsetHigh puts the locked byte at offset 1<<62, far past any content
// the lock file will ever hold. Windows byte-range locks are mandatory, so a
// lock overlapping the pid at offset 0 would make readPID fail with
// ERROR_LOCK_VIOLATION in exactly the process that needs to name the holder.
const lockOffsetHigh = 1 << 30

// lockRange addresses that sentinel byte. Lock and unlock must name the same
// range, so both go through here.
func lockRange() *windows.Overlapped {
	return &windows.Overlapped{OffsetHigh: lockOffsetHigh}
}

// lockFile takes an exclusive byte-range lock that fails immediately when the
// range is already held. One byte is enough: the range is a token, not data.
func lockFile(f *os.File) error {
	return windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, lockRange(),
	)
}

func unlockFile(f *os.File) error {
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, lockRange())
}
