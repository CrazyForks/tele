package keys

import (
	"fmt"
	"sort"
	"strings"
)

// contextOrder is the stable order contexts are rendered in for the config dump.
var contextOrder = []Context{
	ContextGlobal, ContextFolders, ContextChatList, ContextChat,
	ContextComposer, ContextSearch, ContextContextMenu, ContextDeleteSubMenu,
}

// DefaultKeybindingsYAML renders the default keymap as a fully commented-out
// YAML `keybindings:` block (context -> action -> keys). It is embedded in the
// generated config so users can uncomment and edit any binding from its current
// default. Generated from DefaultKeyMap so it never drifts out of sync.
func DefaultKeybindingsYAML() string {
	km := DefaultKeyMap()

	var b strings.Builder
	b.WriteString("# keybindings:                # uncomment a line to override an action's default keys\n")
	for _, ctx := range contextOrder {
		binds := km[ctx]
		if len(binds) == 0 {
			continue
		}

		// Invert key -> action into action -> []key.
		byAction := make(map[Action][]string, len(binds))
		for key, act := range binds {
			byAction[act] = append(byAction[act], key)
		}

		actions := make([]string, 0, len(byAction))
		for act := range byAction {
			actions = append(actions, string(act))
		}
		sort.Strings(actions)

		fmt.Fprintf(&b, "#   %s:\n", ctx)
		for _, act := range actions {
			keyList := byAction[Action(act)]
			sort.Strings(keyList)
			fmt.Fprintf(&b, "#     %s: %s\n", act, formatKeyList(keyList))
		}
	}
	return b.String()
}

// formatKeyList renders a single key as a quoted scalar and multiple keys as a
// YAML flow sequence.
func formatKeyList(keyList []string) string {
	if len(keyList) == 1 {
		return fmt.Sprintf("%q", keyList[0])
	}
	quoted := make([]string, len(keyList))
	for i, k := range keyList {
		quoted[i] = fmt.Sprintf("%q", k)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
