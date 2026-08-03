package components

import (
	"image/color"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/sorokin-vladimir/tele/internal/domain"
)

var (
	inNameStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	editNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	tsStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	sentStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	readStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	// outboxStyle marks a send still on its way: the same weight as the sent
	// tick it will become. failedStyle is the only red in a bubble border.
	outboxStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	failedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	indicatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	quoteStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	sepStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	unreadSepStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
)

var senderPalette = [8]struct{ light, dark color.Color }{
	{lipgloss.Color("1"), lipgloss.Color("9")},
	{lipgloss.Color("2"), lipgloss.Color("10")},
	{lipgloss.Color("4"), lipgloss.Color("12")},
	{lipgloss.Color("5"), lipgloss.Color("13")},
	{lipgloss.Color("6"), lipgloss.Color("14")},
	{lipgloss.Color("130"), lipgloss.Color("208")},
	{lipgloss.Color("92"), lipgloss.Color("141")},
	{lipgloss.Color("28"), lipgloss.Color("118")},
}

func (ml *MessageList) senderNameStyle(senderID int64) lipgloss.Style {
	idx := senderID % 8
	if idx < 0 {
		idx = -idx
	}
	pair := senderPalette[idx]
	fg := lipgloss.LightDark(ml.hasDarkBackground)(pair.light, pair.dark)
	return lipgloss.NewStyle().Foreground(fg).Bold(true)
}

func buildReactStr(reactions []domain.Reaction) string {
	if len(reactions) == 0 {
		return ""
	}
	parts := make([]string, 0, len(reactions))
	for _, r := range reactions {
		s := r.Emoji + " " + strconv.Itoa(r.Count)
		if r.IsChosen {
			parts = append(parts, readStyle.Render(s))
		} else {
			parts = append(parts, tsStyle.Render(s))
		}
	}
	sep := tsStyle.Render(" · ")
	return " " + strings.Join(parts, sep) + " "
}
