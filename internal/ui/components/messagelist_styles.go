package components

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/ui/theme"
)

func (ml *MessageList) senderNameStyle(senderID int64) lipgloss.Style {
	pal := theme.T().SenderPalette
	if len(pal) == 0 {
		return lipgloss.NewStyle().Bold(true)
	}
	idx := senderID % int64(len(pal))
	if idx < 0 {
		idx = -idx
	}
	return lipgloss.NewStyle().Foreground(pal[idx]).Bold(true)
}

func buildReactStr(reactions []domain.Reaction) string {
	if len(reactions) == 0 {
		return ""
	}
	parts := make([]string, 0, len(reactions))
	for _, r := range reactions {
		s := r.Emoji + " " + strconv.Itoa(r.Count)
		if r.IsChosen {
			parts = append(parts, theme.S().TickRead.Render(s))
		} else {
			parts = append(parts, theme.S().Timestamp.Render(s))
		}
	}
	sep := theme.S().Timestamp.Render(" · ")
	return " " + strings.Join(parts, sep) + " "
}
