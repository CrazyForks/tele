package core

import (
	"context"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"

	"github.com/sorokin-vladimir/tele/internal/config"
	"github.com/sorokin-vladimir/tele/internal/core/outbox"
	"github.com/sorokin-vladimir/tele/internal/core/project"
	"github.com/sorokin-vladimir/tele/internal/core/state"
	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/mediacache"
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
	deltas   chan project.Delta
	incoming chan Incoming
	failures chan Failure
	typing   chan Typing
	registry *project.Registry
	readyCh  chan struct{}
	onAuthFn func(userID int64, username string)

	// historyLimit is how many messages one backfill fetches, from config.
	historyLimit int

	// ctx bounds the owner's background work (history backfill). It is stored
	// rather than passed because that work is started by a subscription, which
	// has no call context of its own and outlives it either way.
	ctx context.Context

	// fetching guards one in-flight history fetch per subscription: rapid
	// scroll-up would otherwise fire several identical fetches whose duplicate
	// chunks stack into a repeating date range (issue #120).
	fetchMu  sync.Mutex
	fetching map[project.SubID]bool

	// currentChatID is the chat a client currently has open, consulted only by
	// the notification decision. #192 replaces this with a reported focus.
	currentChatID int64

	// media downloads on behalf of clients and owns the disk cache. It is the
	// only holder of file references (#196).
	media *mediaFetcher

	// outbox is the durable send queue. Sends are handed to it and drained by
	// one worker; nothing about a send lives in a client's memory (#193).
	outbox *outbox.Store
	// outboxWake tells the worker to look again without waiting for its timer.
	// Buffered and dropped when full: one pending wake is as good as ten.
	outboxWake chan struct{}
}

func New(cfg *config.Config, log *zap.Logger, st *state.State, client Connection, n Notifier) *Owner {
	o := &Owner{
		cfg:          cfg,
		log:          log,
		state:        st,
		client:       client,
		notifier:     n,
		authFlow:     internaltg.NewAuthFlow(),
		deltas:       make(chan project.Delta, 256),
		incoming:     make(chan Incoming, 32),
		failures:     make(chan Failure, 32),
		typing:       make(chan Typing, 32),
		readyCh:      make(chan struct{}),
		historyLimit: cfg.UI.HistoryLimit,
		ctx:          context.Background(),
		fetching:     make(map[project.SubID]bool),
		outboxWake:   make(chan struct{}, 1),
	}
	// Built from the owner, not from the store alone: the projection reads the
	// send queue too, and the queue arrives later through SetOutbox (#193).
	o.registry = project.NewRegistry(projectionReader{Store: st.Store(), owner: o})
	o.media = newMediaFetcher(client, st, log)
	if client != nil {
		o.events = client.Updates()
	}
	// Every committed mutation rebuilds the subscribed projections, wherever it
	// originated: the update loop, a history backfill, or a command.
	st.OnChange(o.publishChange)
	return o
}

// SetContext bounds the owner's background work. Call before Start.
func (o *Owner) SetContext(ctx context.Context) { o.ctx = ctx }

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

// SetOutbox gives the owner its durable send queue. Call before Start.
//
// It is set after construction rather than passed to New because the queue
// needs the store's database handle, which the app opens on its own schedule.
func (o *Owner) SetOutbox(s *outbox.Store) { o.outbox = s }

// SetMediaCache gives the owner the directory it caches media in. Call before
// Start. The cache is account-scoped and process-owned: two processes evicting
// independently in one directory would fight (#196).
func (o *Owner) SetMediaCache(c *mediacache.Cache) { o.media.cache = c }

// FetchMedia downloads the named media into the owner's cache if it is not
// there already, and returns the path. The client decodes the file; the bytes
// never cross the owner boundary.
//
// The returned file may in principle be evicted before the client opens it. A
// client that cannot open it renders nothing and asks again on the next
// repaint; see mediacache.Cache.Path.
func (o *Owner) FetchMedia(ctx context.Context, chatID int64, msgID int, slot domain.MediaSlot) (string, error) {
	return o.media.Fetch(ctx, chatID, msgID, slot)
}

// SaveMedia streams the named media into destDir, bypassing the cache, and
// returns the path it actually wrote. The owner picks the name: it follows from
// the document's own name or its MIME type, which is domain knowledge rather
// than rendering.
func (o *Owner) SaveMedia(ctx context.Context, chatID int64, msgID int, slot domain.MediaSlot, destDir string) (string, error) {
	return o.media.Save(ctx, chatID, msgID, slot, destDir)
}

// InvalidateMedia drops a cached file a client could not decode, so the next
// fetch downloads it again rather than handing back the same broken entry.
func (o *Owner) InvalidateMedia(chatID int64, msgID int, slot domain.MediaSlot) {
	o.media.Invalidate(chatID, msgID, slot)
}

// Telegram exposes the raw client.
//
// TRANSITIONAL (#190): the UI still calls tg.Client directly for commands,
// queries and media. #193, #194, #195 and #196 absorb most of those calls and
// #198 absorbs the rest and deletes this method. Do not build on it.
func (o *Owner) Telegram() internaltg.Client { return o.client }
