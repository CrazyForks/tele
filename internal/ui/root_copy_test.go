package ui_test

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/ui"
	"github.com/sorokin-vladimir/tele/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyKey_OnTextMessage_WritesTextToClipboard(t *testing.T) {
	var got string
	defer ui.SetClipboardWriterForTest(func(s string) error { got = s; return nil })()

	m, st := newRootOnChat(t)
	st.AppendMessage(domain.Message{ID: 7, ChatID: 1, Text: "hello world", Date: time.Now()})
	nm, _ := applyHistory(t, m, st, 1)
	m = nm.(ui.RootModel)
	m.View() // lay out the message list so the message becomes the selection

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	require.NotNil(t, cmd, "pressing y on a text message must copy it")
	drainMsgs(cmd())
	assert.Equal(t, "hello world", got)
}

func TestCopyMsgRequest_FromContextMenu_WritesTextToClipboard(t *testing.T) {
	var got string
	defer ui.SetClipboardWriterForTest(func(s string) error { got = s; return nil })()

	m, st := newRootOnChat(t)
	st.AppendMessage(domain.Message{ID: 9, ChatID: 1, Text: "from menu", Date: time.Now()})
	nm, _ := applyHistory(t, m, st, 1)
	m = nm.(ui.RootModel)
	m.View()

	_, cmd := m.Update(components.CopyMsgRequest{})
	require.NotNil(t, cmd, "CopyMsgRequest must copy the selected message")
	drainMsgs(cmd())
	assert.Equal(t, "from menu", got)
}

func TestCopyKey_OnMediaOnlyMessage_DoesNotCopy(t *testing.T) {
	var called bool
	defer ui.SetClipboardWriterForTest(func(string) error { called = true; return nil })()

	m, st := newRootOnChat(t)
	st.AppendMessage(domain.Message{ID: 8, ChatID: 1, Date: time.Now(),
		Media: &domain.MediaRef{Kind: domain.MediaPhoto}, Photo: &domain.PhotoRef{ID: 1}})
	nm, _ := applyHistory(t, m, st, 1)
	m = nm.(ui.RootModel)
	m.View()

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd != nil {
		drainMsgs(cmd())
	}
	assert.False(t, called, "a media-only message has no text to copy")
}
