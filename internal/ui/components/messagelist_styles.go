package components

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/ui/theme"
)

func (ml *MessageList) senderNameStyle(senderID int64) lipgloss.Style {
	idx := senderID % 8
	if idx < 0 {
		idx = -idx
	}
	return lipgloss.NewStyle().Foreground(theme.T().SenderPalette[idx]).Bold(true)
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
