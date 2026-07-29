package core

import (
	"context"

	"github.com/sorokin-vladimir/tele/internal/domain"
)

// SetTyping reports a composing action to the peer. It carries no state: the
// indicator is ephemeral and belongs to whoever is watching.
func (o *Owner) SetTyping(ctx context.Context, chatID int64, action domain.TypingAction) error {
	peer, err := o.peer(chatID)
	if err != nil {
		return err
	}
	return o.client.SetTyping(ctx, peer, action)
}

// SaveDraft syncs the composer's unsent text to Telegram, clearing it when text
// is empty. The local draft is stored either way: it is what the user typed, and
// a failed sync must not lose it.
func (o *Owner) SaveDraft(ctx context.Context, chatID int64, text string) error {
	peer, err := o.peer(chatID)
	if err != nil {
		return err
	}
	o.state.ApplyDraft(chatID, text)
	return o.client.SaveDraft(ctx, peer, text)
}
