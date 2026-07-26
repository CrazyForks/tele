package core

import (
	"context"
	"sync/atomic"

	"go.uber.org/zap"

	"github.com/sorokin-vladimir/tele/internal/config"
	"github.com/sorokin-vladimir/tele/internal/core/state"
	"github.com/sorokin-vladimir/tele/internal/store"
	internaltg "github.com/sorokin-vladimir/tele/internal/tg"
)

// Connection is the owner's view of the Telegram client: the whole command
// surface plus the connect call. tg.Client deliberately omits Connect because
// connecting is the owner's business and no caller's; this interface joins the
// two so the owner can be built over a test double.
type Connection interface {
	internaltg.Client
	Connect(ctx context.Context, cfg *config.Config, af *internaltg.AuthFlow, readyCh chan<- struct{}, onAuth func(int64, string)) error
}

// Owner holds the Telegram connection and everything that may only exist once
// per account: the gotd client, the domain state as sole writer on the update
// path, the update loop and the notification decision. Clients attach to it; in
// this release the only client is the TUI in the same process.
type Owner struct {
	cfg      *config.Config
	log      *zap.Logger
	state    *state.State
	client   Connection
	notifier Notifier
	authFlow *internaltg.AuthFlow

	events   <-chan store.Event
	changes  chan state.Change
	readyCh  chan struct{}
	onAuthFn func(userID int64, username string)

	// currentChatID is the chat a client currently has open, consulted only by
	// the notification decision. #192 replaces this with a reported focus.
	currentChatID int64
}

func New(cfg *config.Config, log *zap.Logger, st *state.State, client Connection, n Notifier) *Owner {
	o := &Owner{
		cfg:      cfg,
		log:      log,
		state:    st,
		client:   client,
		notifier: n,
		authFlow: internaltg.NewAuthFlow(),
		changes:  make(chan state.Change, 64),
		readyCh:  make(chan struct{}),
	}
	if client != nil {
		o.events = client.Updates()
	}
	return o
}

// Changes publishes every applied domain change to the attached client.
func (o *Owner) Changes() <-chan state.Change { return o.changes }

// AuthFlow is the login conversation the client drives on the owner's behalf.
func (o *Owner) AuthFlow() *internaltg.AuthFlow { return o.authFlow }

// Ready is closed once the connection is up and authenticated.
func (o *Owner) Ready() <-chan struct{} { return o.readyCh }

// SetCurrentChat records which chat a client has open, for the notification
// decision only. It is never consulted by domain state (#189).
func (o *Owner) SetCurrentChat(id int64) { atomic.StoreInt64(&o.currentChatID, id) }

// SetOnAuth registers a callback fired once the account is known, so a client
// can record the self identity. Set before Start.
func (o *Owner) SetOnAuth(fn func(userID int64, username string)) { o.onAuthFn = fn }

func (o *Owner) onAuth(userID int64, username string) {
	if o.onAuthFn != nil {
		o.onAuthFn(userID, username)
	}
}

// Telegram exposes the raw client.
//
// TRANSITIONAL (#190): the UI still calls tg.Client directly for commands,
// queries and media. #193, #194, #195 and #196 absorb most of those calls and
// #198 absorbs the rest and deletes this method. Do not build on it.
func (o *Owner) Telegram() internaltg.Client { return o.client }
