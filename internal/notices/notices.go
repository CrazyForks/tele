// Package notices holds one-time startup messages about changes a user must
// not miss, and tracks which of them have already been shown.
package notices

import "time"

// Notice is a single message shown once, ever.
//
// ID is a stable string rather than a version, so notices can be added
// independently and someone who skips a release still sees what applies to
// them. Delay is how long dismissal is blocked, making the message
// unskippable; that is only acceptable because it happens once per notice.
type Notice struct {
	ID    string
	Title string
	Body  string
	Delay time.Duration
}

// Pending returns the notices that have not been shown yet, preserving order.
func Pending(all []Notice, seen Seen) []Notice {
	out := make([]Notice, 0, len(all))
	for _, n := range all {
		if !seen.IsSeen(n.ID) {
			out = append(out, n)
		}
	}
	return out
}
