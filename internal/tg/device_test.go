package tg

import (
	"errors"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeviceConfig_UsesHostnameAsDeviceModel(t *testing.T) {
	d := deviceConfig("1.9.1", func() (string, error) { return "sorokin-mbp", nil })

	assert.Equal(t, "sorokin-mbp", d.DeviceModel)
}

// Without a device model gotd sends runtime.Version(), which shows up in the
// session list as "go1.26.0" and identifies nothing (#200). The fallback must
// still say something about the machine rather than the toolchain.
func TestDeviceConfig_FallsBackToGOOSWhenHostnameFails(t *testing.T) {
	d := deviceConfig("1.9.1", func() (string, error) { return "", errors.New("no hostname") })

	assert.Equal(t, runtime.GOOS, d.DeviceModel)
}

func TestDeviceConfig_FallsBackToGOOSWhenHostnameEmpty(t *testing.T) {
	d := deviceConfig("1.9.1", func() (string, error) { return "", nil })

	assert.Equal(t, runtime.GOOS, d.DeviceModel)
}

func TestDeviceConfig_SystemVersionCarriesPlatform(t *testing.T) {
	d := deviceConfig("1.9.1", func() (string, error) { return "host", nil })

	assert.Equal(t, runtime.GOOS+"/"+runtime.GOARCH, d.SystemVersion)
}

// Left unset, gotd reports its own version, so the session list showed the gotd
// release rather than tele's.
func TestDeviceConfig_AppVersionIsTeleVersion(t *testing.T) {
	d := deviceConfig("1.9.1", func() (string, error) { return "host", nil })

	assert.Equal(t, "1.9.1", d.AppVersion)
}

func TestDeviceConfig_BlankAppVersionFallsBackToDev(t *testing.T) {
	d := deviceConfig("", func() (string, error) { return "host", nil })

	assert.Equal(t, "dev", d.AppVersion)
}
