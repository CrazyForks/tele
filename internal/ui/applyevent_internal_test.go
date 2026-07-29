package ui

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sorokin-vladimir/tele/internal/core"
	"github.com/sorokin-vladimir/tele/internal/core/project"
	"github.com/sorokin-vladimir/tele/internal/core/state"
	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/store"
	"github.com/sorokin-vladimir/tele/internal/telerr"
)

// ownerStub is the in-package twin of the ui_test testOwner: one state and one
// projection registry, queueing the deltas a mutation produces so a test can
// drain them into the model synchronously.
type ownerStub struct {
	state    *state.State
	reg      *project.Registry
	queued   []project.Delta
	incoming []core.Incoming
	typing   []core.Typing

	// calls records the commands the UI issued, so a test can assert on what
	// was asked for rather than on how the result was rendered. err is what
	// every command answers with, standing in for a Telegram refusal.
	calls []cmdCall
	err   error
	// participants is what the mention query answers with.
	participants []domain.ChatMember
}

// cmdCall is one command the UI issued through the owner.
type cmdCall struct {
	name   string
	chatID int64
	flag   bool
}

func newOwnerStub(st store.Store) *ownerStub {
	o := &ownerStub{state: state.New(st), reg: project.NewRegistry(st)}
	o.state.OnChange(func(chg state.Change) {
		if chg.Kind == state.ChangeTyping {
			o.typing = append(o.typing, core.Typing{ChatID: chg.ChatID, Label: chg.Typing.Label()})
			return
		}
		o.queued = append(o.queued, o.reg.Refresh()...)
	})
	return o
}

func (o *ownerStub) Subscribe(w project.Window) project.SubID {
	id, ds := o.reg.Subscribe(w)
	o.queued = append(o.queued, ds...)
	return id
}

func (o *ownerStub) MoveWindow(id project.SubID, w project.Window) {
	o.queued = append(o.queued, o.reg.MoveWindow(id, w)...)
}

func (o *ownerStub) Unsubscribe(id project.SubID) { o.reg.Unsubscribe(id) }

func (o *ownerStub) Refresh() { o.queued = append(o.queued, o.reg.Refresh()...) }

// SetMuted mirrors the real owner: it applies the change through state (which
// publishes a delta) and answers with o.err.
func (o *ownerStub) SetMuted(_ context.Context, chatID int64, muted bool) error {
	o.calls = append(o.calls, cmdCall{name: "SetMuted", chatID: chatID, flag: muted})
	if o.err != nil {
		return o.err
	}
	o.state.ApplyMute(chatID, muted)
	return nil
}

func (o *ownerStub) SetArchived(_ context.Context, chatID int64, archived bool) error {
	o.calls = append(o.calls, cmdCall{name: "SetArchived", chatID: chatID, flag: archived})
	if o.err != nil {
		return o.err
	}
	o.state.ApplyArchived(chatID, archived)
	return nil
}

func (o *ownerStub) SetUnreadMark(_ context.Context, chatID int64, unread bool) error {
	o.calls = append(o.calls, cmdCall{name: "SetUnreadMark", chatID: chatID, flag: unread})
	if o.err != nil {
		return o.err
	}
	o.state.ApplyUnreadMark(chatID, unread)
	return nil
}

func (o *ownerStub) SearchContacts(_ context.Context, _ string, _ int) ([]domain.Chat, error) {
	return nil, o.err
}

func (o *ownerStub) GetParticipants(_ context.Context, chatID int64) ([]domain.ChatMember, error) {
	o.calls = append(o.calls, cmdCall{name: "GetParticipants", chatID: chatID})
	return o.participants, o.err
}

func (o *ownerStub) SetTyping(_ context.Context, chatID int64, _ domain.TypingAction) error {
	o.calls = append(o.calls, cmdCall{name: "SetTyping", chatID: chatID})
	return o.err
}

func (o *ownerStub) SaveDraft(_ context.Context, chatID int64, text string) error {
	o.calls = append(o.calls, cmdCall{name: "SaveDraft", chatID: chatID})
	o.state.ApplyDraft(chatID, text)
	return o.err
}

func (o *ownerStub) Forward(_ context.Context, fromChatID int64, to domain.Peer, _ []int, _ string) error {
	o.calls = append(o.calls, cmdCall{name: "Forward", chatID: fromChatID})
	if o.err != nil {
		return o.err
	}
	o.state.Store().BumpChatLastMessage(to.ID, domain.Message{ChatID: to.ID, IsOut: true, Date: time.Now()})
	o.queued = append(o.queued, o.reg.Refresh()...)
	return nil
}

func (o *ownerStub) SendReaction(_ context.Context, chatID int64, msgID int, emoji string) error {
	o.calls = append(o.calls, cmdCall{name: "SendReaction", chatID: chatID})
	if o.err != nil {
		return o.err
	}
	o.state.ApplyReactions(chatID, msgID,
		[]domain.Reaction{{Emoji: emoji, Count: 1, IsChosen: true}}, false)
	return nil
}

func (o *ownerStub) DeleteMessages(_ context.Context, chatID int64, msgIDs []int, _ bool) error {
	o.calls = append(o.calls, cmdCall{name: "DeleteMessages", chatID: chatID})
	removed := make([]domain.Message, 0, len(msgIDs))
	for _, m := range o.state.Store().Messages(chatID) {
		for _, id := range msgIDs {
			if m.ID == id {
				removed = append(removed, m)
			}
		}
	}
	o.state.ApplyDelete(chatID, msgIDs)
	if o.err != nil {
		for _, m := range removed {
			o.state.ApplyRestore(m)
		}
		return o.err
	}
	return nil
}

func (o *ownerStub) EditMessage(_ context.Context, chatID int64, msgID int, text string, entities []domain.MessageEntity) error {
	o.calls = append(o.calls, cmdCall{name: "EditMessage", chatID: chatID})
	var prev domain.Message
	found := false
	for _, m := range o.state.Store().Messages(chatID) {
		if m.ID == msgID {
			prev, found = m, true
			break
		}
	}
	if !found {
		return &telerr.Error{Kind: telerr.NotFound}
	}
	if o.err != nil {
		return o.err
	}
	edited := prev
	edited.Text = text
	edited.Entities = entities
	now := time.Now()
	edited.EditDate = &now
	o.state.ApplyEdit(edited)
	return nil
}

func (o *ownerStub) ReadReactions(_ context.Context, chatID int64) error {
	o.calls = append(o.calls, cmdCall{name: "ReadReactions", chatID: chatID})
	if o.err != nil {
		return o.err
	}
	o.state.ApplyReactionsRead(chatID)
	return nil
}

func (o *ownerStub) ReadMentions(_ context.Context, chatID int64) error {
	o.calls = append(o.calls, cmdCall{name: "ReadMentions", chatID: chatID})
	if o.err != nil {
		return o.err
	}
	o.state.ApplyMentionsRead(chatID)
	return nil
}

func (o *ownerStub) MarkRead(_ context.Context, chatID int64, maxID int) error {
	o.calls = append(o.calls, cmdCall{name: "MarkRead", chatID: chatID})
	if o.err != nil {
		return o.err
	}
	if maxID == 0 {
		o.state.ApplyChatRead(chatID)
		return nil
	}
	o.state.ApplyReadInbox(chatID, maxID)
	return nil
}

func (o *ownerStub) AddToFolder(_ context.Context, filterID int, chatID int64, add bool) error {
	o.calls = append(o.calls, cmdCall{name: "AddToFolder", chatID: chatID, flag: add})
	if o.err != nil {
		return o.err
	}
	o.state.ApplyFolderMembership(filterID, chatID, add)
	return nil
}

func (o *ownerStub) drain(m RootModel) (tea.Model, tea.Cmd) {
	var model tea.Model = m
	var cmd tea.Cmd
	for len(o.queued) > 0 || len(o.incoming) > 0 || len(o.typing) > 0 {
		deltas, events, typing := o.queued, o.incoming, o.typing
		o.queued, o.incoming, o.typing = nil, nil, nil
		for _, d := range deltas {
			model, cmd = model.Update(d)
		}
		for _, in := range events {
			model, cmd = model.Update(in)
		}
		for _, tp := range typing {
			model, cmd = model.Update(tp)
		}
	}
	return model, cmd
}

// newRootInternal builds a model wired to the stub owner, as app.Run wires the
// real one.
func newRootInternal(st store.Store, historyLimit int) RootModel {
	m := NewRootModel(nil, st, historyLimit, false)
	if st != nil {
		m = m.WithOwner(newOwnerStub(st))
	}
	return m
}

// applyEventInternal is the in-package twin of applyEvent in the ui_test
// package: it applies a raw Telegram event to domain state and drains the
// resulting projection deltas into the model, as the owner's update loop and the
// delta pump do in production.
func applyEventInternal(t *testing.T, m RootModel, st store.Store, evt store.Event) (tea.Model, tea.Cmd) {
	t.Helper()
	o, ok := m.owner.(*ownerStub)
	if !ok {
		state.Apply(state.New(st), evt)
		return m, nil
	}
	chg, applied := state.Apply(o.state, evt)
	if applied && chg.Kind == state.ChangeNewMessage && !chg.Message.IsOut &&
		chg.ChatID != m.currentChatID {
		o.incoming = append(o.incoming, incomingLike(m, st, chg))
	}
	return o.drain(m)
}

// incomingLike reproduces core.Owner.publishIncoming: the same freshness and
// mute gate, and the same privacy rule on the preview. Keeping the two in step
// is what makes a UI test about notifications mean anything.
func incomingLike(m RootModel, st store.Store, chg state.Change) core.Incoming {
	chat, _ := st.GetChat(chg.ChatID)
	preview := chg.Message.Text
	if m.cfg != nil && !m.cfg.UI.NotificationPreview {
		preview = "New message"
	}
	return core.Incoming{
		ChatID:  chg.ChatID,
		Title:   chat.Title,
		Preview: preview,
		Notify:  store.Notifiable(st, chg.ChatID, m.currentChatID, chg.Message.Date, time.Now()),
	}
}
