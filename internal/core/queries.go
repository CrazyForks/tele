package core

import (
	"context"

	"github.com/sorokin-vladimir/tele/internal/domain"
)

// SearchContacts asks Telegram for users matching q. It is a query, not a
// projection: the result is a one-off answer nobody subscribes to. No chat ID
// is involved because the point is finding chats the owner may not hold.
func (o *Owner) SearchContacts(ctx context.Context, q string, limit int) ([]domain.Chat, error) {
	return o.client.SearchContacts(ctx, q, limit)
}

// GetParticipants returns mention candidates for a group or channel.
func (o *Owner) GetParticipants(ctx context.Context, chatID int64) ([]domain.ChatMember, error) {
	peer, err := o.peer(chatID)
	if err != nil {
		return nil, err
	}
	return o.client.GetParticipants(ctx, peer)
}
