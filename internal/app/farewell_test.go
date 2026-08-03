package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFarewellBanner_FullInfo(t *testing.T) {
	out := farewellBanner("1.2.3", "@sorokin", "/home/u/.local/state/tele/tele.log")

	assert.Contains(t, out, `\__|  \___| |_|  \___|`, "the splash logo art must be reused verbatim")
	assert.Contains(t, out, "v1.2.3 · @sorokin")
	assert.Contains(t, out, "log  /home/u/.local/state/tele/tele.log")
}

func TestFarewellBanner_NoAccount_OmitsIt(t *testing.T) {
	out := farewellBanner("1.2.3", "", "/var/log/tele.log")

	assert.Contains(t, out, "v1.2.3")
	assert.NotContains(t, out, "·", "without an account the version stands alone, no dangling separator")
}

func TestFarewellBanner_NoLogPath_OmitsLine(t *testing.T) {
	out := farewellBanner("1.2.3", "@sorokin", "")

	assert.NotContains(t, out, "log ")
}

func TestFarewellBanner_DevBuild(t *testing.T) {
	out := farewellBanner("dev", "", "")

	assert.Contains(t, out, "dev")
	assert.NotContains(t, out, "vdev")
}

func TestFarewellBanner_EndsWithBlankLine(t *testing.T) {
	out := farewellBanner("1.2.3", "", "")

	// The shell prompt returns right after this, so the banner leaves one blank
	// line rather than butting straight against it.
	assert.True(t, strings.HasSuffix(out, "\n\n"), "want trailing blank line, got %q", out)
}

func TestShortenHome(t *testing.T) {
	assert.Equal(t, "~/.local/state/tele/tele.log", shortenHome("/home/u/.local/state/tele/tele.log", "/home/u"))
	// A path outside the home directory is left alone, and so is a prefix match
	// that is not a path boundary.
	assert.Equal(t, "/var/log/tele.log", shortenHome("/var/log/tele.log", "/home/u"))
	assert.Equal(t, "/home/user2/x", shortenHome("/home/user2/x", "/home/u"))
	// No home known: nothing to shorten.
	assert.Equal(t, "/home/u/x", shortenHome("/home/u/x", ""))
}

func TestAccountLabel(t *testing.T) {
	assert.Equal(t, "@sorokin", accountLabel(42, "sorokin"))
	assert.Equal(t, "id 42", accountLabel(42, ""), "accounts without a username still identify by id")
	assert.Equal(t, "", accountLabel(0, ""), "no authenticated account yet")
}

func TestApp_SelfIdentity_RecordedFromAuth(t *testing.T) {
	var a App
	a.setSelf(42, "sorokin")

	id, name := a.self()
	require.Equal(t, int64(42), id)
	assert.Equal(t, "sorokin", name)
}
