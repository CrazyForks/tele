package keys_test

import (
	"strings"
	"testing"

	"github.com/sorokin-vladimir/tele/internal/ui/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultKeybindingsYAML_IsFullyCommented(t *testing.T) {
	out := keys.DefaultKeybindingsYAML()
	require.NotEmpty(t, out)
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		assert.True(t, strings.HasPrefix(line, "#"),
			"every line must be commented out so defaults stay active: %q", line)
	}
}

func TestDefaultKeybindingsYAML_ContainsContextsAndActions(t *testing.T) {
	out := keys.DefaultKeybindingsYAML()
	assert.Contains(t, out, "# keybindings:")
	assert.Contains(t, out, "#   chat:")
	assert.Contains(t, out, `#     reply: "r"`)
	assert.Contains(t, out, `#     play_voice: "p"`)
	// An action with several default keys renders as a list.
	assert.Regexp(t, `#     (open_context_menu|down): \[`, out)
}
