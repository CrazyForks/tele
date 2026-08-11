// Package themes holds the ported palettes tele ships with, and the embedded
// copy of them the binary resolves by name.
//
// The directory is the source: it is what a contributor sends a pull request
// against, and each file carries the palette it was ported from and the places
// it had to depart from it. Embedding reads that directory rather than
// replacing it, and the pattern is a wildcard on purpose — a palette added here
// is in the binary without a second list to keep in step.
//
// A theme in here is bundled, not built-in. Nothing depends on any of these
// existing: they resolve by name on a machine with no theme files at all, and a
// user theme of the same name replaces one outright. The two themes that must
// always be there, tele-dark and tele-light, are compiled from Go in
// internal/ui/theme.
package themes

import "embed"

// FS is the bundled themes, as a file system the loader reads with the same
// code it reads the user's themes directory with.
//
//go:embed *.yml
var FS embed.FS
