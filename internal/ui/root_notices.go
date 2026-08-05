package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sorokin-vladimir/tele/internal/notices"
	"github.com/sorokin-vladimir/tele/internal/ui/components"
)

// noticeTickMsg advances the dismissal countdown once per second.
type noticeTickMsg struct{}

// WithNotices seeds the pending one-time notices. Seen-state is written on
// dismissal, not on display, so quitting early shows the notice again.
func (m RootModel) WithNotices(pending []notices.Notice, seen notices.Seen) RootModel {
	m.noticeQueue = pending
	m.noticeSeen = seen
	m.noticeLeft = 0
	if len(pending) > 0 {
		m.noticeLeft = int(pending[0].Delay.Seconds())
	}
	return m
}

func (m RootModel) noticeActive() bool { return len(m.noticeQueue) > 0 }

func (m RootModel) noticeRemaining() int { return m.noticeLeft }

// noticeTick decrements the countdown, never below zero.
func (m RootModel) noticeTick() RootModel {
	if m.noticeLeft > 0 {
		m.noticeLeft--
	}
	return m
}

// noticeTickCmd schedules the next countdown tick.
func noticeTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return noticeTickMsg{} })
}

// handleNoticeKey consumes a key press while a notice is up. The bool reports
// whether the key was swallowed, so the caller knows not to process it further.
// Before the countdown ends every key is swallowed and nothing happens; after
// it, any key dismisses the notice and advances the queue.
func (m RootModel) handleNoticeKey() (RootModel, bool) {
	if !m.noticeActive() {
		return m, false
	}
	if m.noticeLeft > 0 {
		return m, true
	}
	m.noticeSeen.MarkSeen(m.noticeQueue[0].ID)
	m.noticeQueue = m.noticeQueue[1:]
	m.noticeLeft = 0
	if len(m.noticeQueue) > 0 {
		m.noticeLeft = int(m.noticeQueue[0].Delay.Seconds())
	}
	return m, true
}

// noticeView renders the head of the queue centred over dimmed content.
func (m RootModel) noticeView(content string) string {
	n := m.noticeQueue[0]
	maxW := m.width - 10
	if maxW > 72 {
		maxW = 72
	}
	if maxW < 24 {
		maxW = 24
	}
	box := components.RenderNoticeBox(n.Title, n.Body, m.noticeLeft, maxW)
	return overlayCenter(dimBackground(content), box, m.width, m.height)
}
