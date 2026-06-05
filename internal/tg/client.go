package tg

import (
	"context"
	"sync"

	"go.uber.org/zap"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"

	"github.com/sorokin-vladimir/tele/internal/config"
	"github.com/sorokin-vladimir/tele/internal/store"
)

// GotdClient wraps the gotd telegram client and implements the Client interface
type GotdClient struct {
	mu           sync.RWMutex
	api          *tg.Client
	mustDeliver  chan store.Event
	droppable    chan store.Event
	updates      chan store.Event
	peers        map[int64]store.Peer
	log          *zap.Logger
	traceLog     *zap.Logger
	suppressMu   sync.Mutex
	suppressIDs  map[int]struct{}
	stateStorage updates.StateStorage
}

func NewGotdClient(log *zap.Logger, stateStorage updates.StateStorage, trace bool) *GotdClient {
	traceLog := zap.NewNop()
	if trace {
		traceLog = log
	}
	return &GotdClient{
		mustDeliver:  make(chan store.Event, 256),
		droppable:    make(chan store.Event, 64),
		updates:      make(chan store.Event, 32),
		peers:        make(map[int64]store.Peer),
		log:          log,
		traceLog:     traceLog,
		suppressIDs:  make(map[int]struct{}),
		stateStorage: stateStorage,
	}
}

// Connect starts the gotd client. Call in a goroutine — blocks until ctx is cancelled.
// Closes readyCh once auth is complete and the updates loop has started.
// onAuth is called with the authenticated user ID before readyCh is closed; may be nil.
func (c *GotdClient) Connect(ctx context.Context, cfg *config.Config, af *AuthFlow, readyCh chan<- struct{}, onAuth func(int64)) error {
	sess := NewFileSession(cfg.Telegram.SessionFile)

	dispatcher := tg.NewUpdateDispatcher()
	setupDispatcher(&dispatcher, c.mustDeliver, c.droppable, c.log, func(id int) bool {
		c.suppressMu.Lock()
		defer c.suppressMu.Unlock()
		if _, ok := c.suppressIDs[id]; ok {
			delete(c.suppressIDs, id)
			return true
		}
		return false
	})

	// updates.New does not return an error — confirmed via go doc.
	manager := updates.New(updates.Config{
		Handler: dispatcher,
		Storage: c.stateStorage,
	})

	// outboxHook intercepts UpdateReadHistoryOutbox / UpdateReadChannelOutbox before
	// the pts-tracking layer. updates.Manager silently drops these when a pts gap is
	// present (the pending buffer never flushes), so we extract them from the raw
	// wire message and emit the event immediately, then hand the update on to the
	// manager as usual.
	hook := newOutboxHook(manager, c.mustDeliver, c.log)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case evt := <-c.mustDeliver:
				select {
				case c.updates <- evt:
				case <-ctx.Done():
					return
				}
			case evt := <-c.droppable:
				select {
				case c.updates <- evt:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	tc := telegram.NewClient(cfg.Telegram.APIID, cfg.Telegram.APIHash, telegram.Options{
		UpdateHandler:  hook,
		SessionStorage: sess,
		Logger:         c.log,
	})

	c.log.Debug("connecting to telegram")
	return tc.Run(ctx, func(ctx context.Context) error {
		c.log.Debug("running auth flow")
		flow := auth.NewFlow(af, auth.SendCodeOptions{})
		if err := tc.Auth().IfNecessary(ctx, flow); err != nil {
			c.log.Error("auth failed", zap.Error(err))
			return err
		}

		self, err := tc.Self(ctx)
		if err != nil {
			c.log.Error("Self() failed", zap.Error(err))
			return err
		}
		c.log.Info("authenticated", zap.Int64("user_id", self.ID))

		if onAuth != nil {
			onAuth(self.ID)
		}

		c.mu.Lock()
		c.api = tc.API()
		c.mu.Unlock()

		return manager.Run(ctx, tc.API(), self.ID, updates.AuthOptions{
			OnStart: func(ctx context.Context) {
				c.log.Debug("updates manager started, signalling ready")
				close(readyCh)
			},
		})
	})
}

func (c *GotdClient) Updates() <-chan store.Event {
	return c.updates
}
