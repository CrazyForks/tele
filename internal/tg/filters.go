package tg

import (
	"context"

	"github.com/gotd/td/tg"
	"go.uber.org/zap"

	"github.com/sorokin-vladimir/tele/internal/store"
)

func (c *GotdClient) GetDialogFilters(ctx context.Context) ([]store.FolderFilter, error) {
	api, err := c.acquireAPI()
	if err != nil {
		return nil, err
	}

	var filters []store.FolderFilter
	err = WithRetry(ctx, func() error {
		result, err := api.MessagesGetDialogFilters(ctx)
		if err != nil {
			c.log.Error("MessagesGetDialogFilters failed", zap.Error(err))
			return err
		}
		filters = parseDialogFilters(result.Filters)
		return nil
	})
	return filters, err
}

func parseDialogFilters(raw []tg.DialogFilterClass) []store.FolderFilter {
	var out []store.FolderFilter
	for _, f := range raw {
		df, ok := f.(*tg.DialogFilter)
		if !ok {
			// Skip DialogFilterDefault (All Chats sentinel) and DialogFilterChatlist
			continue
		}
		out = append(out, store.FolderFilter{
			ID:              df.ID,
			Title:           df.Title.Text,
			Emoji:           df.Emoticon,
			PinnedPeers:     extractPeerIDs(df.PinnedPeers),
			IncludePeers:    extractPeerIDs(df.IncludePeers),
			ExcludePeers:    extractPeerIDs(df.ExcludePeers),
			Contacts:        df.Contacts,
			NonContacts:     df.NonContacts,
			Groups:          df.Groups,
			Broadcasts:      df.Broadcasts,
			Bots:            df.Bots,
			ExcludeMuted:    df.ExcludeMuted,
			ExcludeRead:     df.ExcludeRead,
			ExcludeArchived: df.ExcludeArchived,
		})
	}
	return out
}

func extractPeerIDs(peers []tg.InputPeerClass) []int64 {
	ids := make([]int64, 0, len(peers))
	for _, p := range peers {
		switch v := p.(type) {
		case *tg.InputPeerUser:
			ids = append(ids, v.UserID)
		case *tg.InputPeerChat:
			ids = append(ids, v.ChatID)
		case *tg.InputPeerChannel:
			ids = append(ids, v.ChannelID)
		}
	}
	return ids
}
