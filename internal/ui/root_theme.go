package ui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sorokin-vladimir/tele/internal/ui/components"
	"github.com/sorokin-vladimir/tele/internal/ui/theme"
)

// reloadThemes re-reads the theme files and installs them. It is cheap and safe
// mid-session: no component caches a style, and the one cache there is holds
// line counts, which a color cannot change. The applied background is kept, so
// reloading at noon does not snap the app to the dark slot.
//
// Its reason to exist is the authoring loop. Writing a theme means changing one
// of sixty tokens and looking; through a restart each look costs a reconnect.
func (m RootModel) reloadThemes() (RootModel, tea.Cmd) {
	if m.cfg == nil {
		return m, nil
	}
	loaded := theme.LoadSlots(m.cfg.ThemesDir, m.cfg.UI.ThemeSlots.Dark, m.cfg.UI.ThemeSlots.Light)
	theme.SetSlots(loaded.Slots())

	kind, text := components.ToastInfo, fmt.Sprintf("themes reloaded: %s / %s",
		loaded.Dark.Theme.Name, loaded.Light.Theme.Name)
	if len(loaded.Warnings) > 0 {
		// The first problem is the one worth reading; the rest are in the log,
		// and a reload is something you repeat until it is clean anyway.
		kind, text = components.ToastWarning, loaded.Warnings[0]
		if m.log != nil {
			for _, w := range loaded.Warnings {
				m.log.Warn("theme: " + w)
			}
		}
	}
	serial := m.toasts.Add(kind, text)
	return m, tea.Tick(5*time.Second, func(time.Time) tea.Msg {
		return ClearStatusErrMsg{Serial: serial}
	})
}
