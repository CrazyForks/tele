// Package version holds the application version, injected at build time.
//
// It exists as its own package because more than one place needs the version:
// the -version flag in cmd/tele, and the initConnection parameters that name
// this app in Telegram's active-sessions list (#200). internal packages cannot
// import main, so the value cannot live there.
package version

// Version is the release this binary was built from. Release builds inject it
// via -ldflags "-X .../internal/version.Version=1.2.3"; local builds keep "dev".
var Version = "dev"
