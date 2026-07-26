package tg

import (
	"runtime"

	"github.com/gotd/td/telegram"
)

// deviceConfig builds the initConnection parameters Telegram records with a
// session and shows in the active-sessions list.
//
// Left empty, gotd fills its own defaults: runtime.Version() as the device
// model, runtime.GOOS as the system version and its own release as the app
// version, which rendered as "go1.26.0 / tele app v0.160.0, Desktop darwin" —
// the Go toolchain and the gotd release, identifying neither this app nor the
// machine. The session list is where someone decides whether a session is
// theirs and whether to terminate it, so it has to say something they can act
// on (#200).
//
// hostname is injected so the fallback path is testable; production passes
// os.Hostname.
func deviceConfig(appVersion string, hostname func() (string, error)) telegram.DeviceConfig {
	model, err := hostname()
	if err != nil || model == "" {
		model = runtime.GOOS
	}
	if appVersion == "" {
		appVersion = "dev"
	}
	return telegram.DeviceConfig{
		DeviceModel:   model,
		SystemVersion: runtime.GOOS + "/" + runtime.GOARCH,
		AppVersion:    appVersion,
	}
}
