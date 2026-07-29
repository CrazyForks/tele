package core

import "context"

// SetMuted mutes or unmutes a chat. The new state is applied before the request
// so every attached client sees it at once, and undone if Telegram refuses.
func (o *Owner) SetMuted(ctx context.Context, chatID int64, muted bool) error {
	peer, err := o.peer(chatID)
	if err != nil {
		return err
	}
	if _, changed := o.state.ApplyMute(chatID, muted); !changed {
		return nil
	}
	if err := o.client.SetMuted(ctx, peer, muted); err != nil {
		o.state.ApplyMute(chatID, !muted)
		return err
	}
	return nil
}

// SetArchived moves a chat into or out of the Archive folder.
func (o *Owner) SetArchived(ctx context.Context, chatID int64, archived bool) error {
	peer, err := o.peer(chatID)
	if err != nil {
		return err
	}
	if _, changed := o.state.ApplyArchived(chatID, archived); !changed {
		return nil
	}
	if err := o.client.SetArchived(ctx, peer, archived); err != nil {
		o.state.ApplyArchived(chatID, !archived)
		return err
	}
	return nil
}

// MarkRead reports messages up to maxID as read. A maxID of 0 means the whole
// chat, which is what the chat menu's "mark as read" asks for.
//
// Unlike the flag commands this is not optimistic: the pointer moves only after
// Telegram confirms, because an unread count running ahead of the server would
// be wrong in the direction a user notices.
func (o *Owner) MarkRead(ctx context.Context, chatID int64, maxID int) error {
	peer, err := o.peer(chatID)
	if err != nil {
		return err
	}
	if err := o.client.MarkRead(ctx, peer, maxID); err != nil {
		return err
	}
	if maxID == 0 {
		o.state.ApplyChatRead(chatID)
		return nil
	}
	o.state.ApplyReadInbox(chatID, maxID)
	return nil
}

// ReadReactions marks every unread reaction in a chat as read. Like MarkRead it
// clears state only after Telegram confirms.
func (o *Owner) ReadReactions(ctx context.Context, chatID int64) error {
	peer, err := o.peer(chatID)
	if err != nil {
		return err
	}
	if err := o.client.ReadReactions(ctx, peer); err != nil {
		return err
	}
	o.state.ApplyReactionsRead(chatID)
	return nil
}

// ReadMentions marks every unread mention in a chat as read.
func (o *Owner) ReadMentions(ctx context.Context, chatID int64) error {
	peer, err := o.peer(chatID)
	if err != nil {
		return err
	}
	if err := o.client.ReadMentions(ctx, peer); err != nil {
		return err
	}
	o.state.ApplyMentionsRead(chatID)
	return nil
}

// AddToFolder adds or removes a chat from a folder filter.
func (o *Owner) AddToFolder(ctx context.Context, filterID int, chatID int64, add bool) error {
	peer, err := o.peer(chatID)
	if err != nil {
		return err
	}
	if _, changed := o.state.ApplyFolderMembership(filterID, chatID, add); !changed {
		return nil
	}
	if err := o.client.AddToFolder(ctx, filterID, peer, add); err != nil {
		o.state.ApplyFolderMembership(filterID, chatID, !add)
		return err
	}
	return nil
}

// SetUnreadMark sets or clears the manual unread mark on a chat.
func (o *Owner) SetUnreadMark(ctx context.Context, chatID int64, unread bool) error {
	peer, err := o.peer(chatID)
	if err != nil {
		return err
	}
	if _, changed := o.state.ApplyUnreadMark(chatID, unread); !changed {
		return nil
	}
	if err := o.client.MarkDialogUnread(ctx, peer, unread); err != nil {
		o.state.ApplyUnreadMark(chatID, !unread)
		return err
	}
	return nil
}
