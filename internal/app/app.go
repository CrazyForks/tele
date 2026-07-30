package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"go.uber.org/zap"

	"github.com/sorokin-vladimir/tele/internal/config"
	"github.com/sorokin-vladimir/tele/internal/core"
	"github.com/sorokin-vladimir/tele/internal/core/state"
	"github.com/sorokin-vladimir/tele/internal/notices"
	"github.com/sorokin-vladimir/tele/internal/store"
	internaltg "github.com/sorokin-vladimir/tele/internal/tg"
	"github.com/sorokin-vladimir/tele/internal/ui"
	"github.com/sorokin-vladimir/tele/internal/ui/components"
	"github.com/sorokin-vladimir/tele/internal/ui/keys"
	"github.com/sorokin-vladimir/tele/internal/ui/screens"
)

type App struct {
	cfg   *config.Config
	log   *zap.Logger
	st    store.Store
	owner *core.Owner
	// sqlite is the same object as st, kept concretely because notice
	// seen-state needs the database handle and store.Store does not expose it.
	sqlite *store.SQLiteStore
	// tmpDir holds this run's scratch files: media saved for an external
	// player, GIFs staged for decoding, and the media cache when the user asked
	// for no persistent one. Removed on exit.
	tmpDir  string
	verbose bool
	// stateMoved reports that startup migration relocated the account state, so
	// the user can be told where it went.
	stateMoved bool
}

// SetStateMoved records whether startup migration relocated the account state.
func (a *App) SetStateMoved(moved bool) { a.stateMoved = moved }

// pendingNotices lists the one-time startup messages this build can show.
// Conditional entries are omitted when they do not apply, so a user only ever
// sees what actually happened on their machine.
func (a *App) pendingNotices() []notices.Notice {
	const delay = 7 * time.Second
	out := []notices.Notice{
		{
			ID:    "single-instance-v1.10",
			Title: "Only one tele at a time",
			Delay: delay,
			Body: "Starting a second tele on the same account now fails with a message " +
				"instead of starting. Two instances shared one session and one database " +
				"with nothing arbitrating between them, quietly overwriting each other's " +
				"unread counts and sync state, which surfaced later as missed messages.",
		},
	}
	if a.stateMoved {
		out = append(out, notices.Notice{
			ID:    "state-dir-moved-v1.10",
			Title: "Your data moved",
			Delay: delay,
			Body: "The session and local database now live in " + a.cfg.StateDir +
				", instead of next to the config file. They were moved for you and " +
				"nothing was lost: you are still logged in. The old location is now empty " +
				"and can be ignored.",
		})
	}
	if a.cfg.SessionPinned {
		out = append(out, notices.Notice{
			ID:    "session-file-deprecated-v1.10",
			Title: "session_file is going away",
			Delay: delay,
			Body: "Your config sets telegram.session_file, so your session was left exactly " +
				"where it is. That setting is deprecated and will be removed in the next " +
				"release. Replace it with state_dir pointing at the directory that should " +
				"hold the session and the database.",
		})
	}
	return out
}

func New(cfg *config.Config, log *zap.Logger, verbose bool, trace bool) (*App, error) {
	statePath := filepath.Join(cfg.StateDir, "state.db")
	sqliteStore, err := store.NewSQLite(statePath, log)
	if err != nil {
		return nil, fmt.Errorf("open state DB: %w", err)
	}
	stateStorage := internaltg.NewSQLiteStateStorage(sqliteStore.DB())
	client := internaltg.NewGotdClient(log, stateStorage, trace)
	owner := core.New(cfg, log, state.New(sqliteStore), client, newNotifier(log))
	owner.SetOnAuth(func(userID int64, username string) {
		components.SetSelfIdentity(userID, username)
	})

	// The temp directory is created here rather than in Run because the media
	// cache may live inside it, and the owner needs the cache before it starts.
	tmpDir, err := os.MkdirTemp("", "tele-*")
	if err != nil {
		log.Warn("failed to create temp dir for media", zap.Error(err))
		tmpDir = ""
	}
	removeLegacyMediaCache(log)
	if cache, cerr := openMediaCache(cfg, tmpDir, log); cerr != nil {
		log.Warn("media cache unavailable; media will not be cached", zap.Error(cerr))
	} else {
		owner.SetMediaCache(cache)
	}

	return &App{
		cfg:     cfg,
		log:     log,
		st:      sqliteStore,
		sqlite:  sqliteStore,
		owner:   owner,
		tmpDir:  tmpDir,
		verbose: verbose,
	}, nil
}

func (a *App) Run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if sc, ok := a.st.(interface{ Close() error }); ok {
		defer func() { _ = sc.Close() }()
	}

	defer os.RemoveAll(a.tmpDir) //nolint:errcheck

	authFlow := a.owner.AuthFlow()
	readyCh := a.owner.Ready()

	// The owner holds the connection; this process also happens to render it.
	tgErr := make(chan error, 1)
	a.owner.SetContext(ctx)
	go func() { tgErr <- a.owner.Start(ctx) }()
	go a.owner.RunUpdates(ctx)

	// Build bubbletea model
	km, warns := keys.MergeOverrides(keys.DefaultKeyMap(), a.cfg.KeybindingOverrides())
	for _, w := range warns {
		a.log.Warn("keybindings: " + w)
	}
	root := ui.NewRootModel(a.owner.Telegram(), a.st, a.cfg.UI.HistoryLimit, a.verbose)
	root = root.WithContext(ctx).WithConfig(a.cfg).WithKeyMap(km).WithOwner(a.owner).WithLogger(a.log)
	root.SetLoginModel(screens.NewLoginModel(authFlow))
	root.SetOnChatOpen(func(id int64) {
		a.owner.SetCurrentChat(id)
	})
	root.SetTmpDir(a.tmpDir)

	// One-time startup notices (#197). Seen-state is written on dismissal, so
	// quitting before the countdown ends shows the notice again next time.
	noticeSeen := notices.NewSQLiteSeen(a.sqlite.DB())
	root = root.WithNotices(notices.Pending(a.pendingNotices(), noticeSeen), noticeSeen)

	prog := tea.NewProgram(root)

	// Bridge: auth requests + ready signal → bubbletea
	go func() {
		var authOK bool
		for {
			cmd := screens.WaitForAuthRequest(authFlow, readyCh)
			msg := cmd()
			prog.Send(msg)
			if req, isReq := msg.(screens.AuthRequestMsg); isReq {
				a.log.Debug("auth step requested", zap.Int("step", int(req.Step)))
			}
			if _, done := msg.(screens.ConnectedMsg); done {
				a.log.Info("connected, loading dialogs")
				authOK = true
				break
			}
			if errMsg, failed := msg.(screens.AuthErrorMsg); failed {
				a.log.Error("auth error", zap.String("reason", errMsg.Text))
				break
			}
		}
		if !authOK {
			return
		}
		// Connected: the owner loads the authoritative dialog list.
		go func() {
			if err := a.owner.Bootstrap(ctx); err != nil {
				a.log.Error("GetDialogs failed", zap.Error(err))
				return
			}
			prog.Send(screens.TransitionToMainMsg{})
		}()

		// Send cached folder filters immediately, then refresh from network
		if cached := a.st.FolderFilters(); len(cached) > 0 {
			prog.Send(ui.FolderFiltersMsg{Filters: cached})
		}
		go func() {
			filters, err := a.owner.LoadFolderFilters(ctx)
			if err != nil {
				a.log.Warn("GetDialogFilters failed", zap.Error(err))
				return
			}
			if len(filters) == 0 {
				return
			}
			prog.Send(ui.FolderFiltersMsg{Filters: filters})
		}()
	}()

	// Bridge: projection deltas → bubbletea
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case d := <-a.owner.Deltas():
				prog.Send(d)
			case in := <-a.owner.Incoming():
				prog.Send(in)
			case f := <-a.owner.Failures():
				prog.Send(f)
			case tp := <-a.owner.Typing():
				prog.Send(tp)
			}
		}
	}()

	_, err := prog.Run()
	cancel()

	// Disable OS color-scheme reports (DEC mode 2031) enabled at startup, so the
	// terminal stops emitting report sequences to the shell after tele exits
	// (issue #148). The program has restored the normal screen by now, so write
	// the reset directly.
	_, _ = fmt.Fprint(os.Stdout, ansi.ResetModeLightDark)

	// Wait for tg client goroutine
	tgClientErr := <-tgErr
	if tgClientErr != nil && err == nil {
		return fmt.Errorf("telegram: %w", tgClientErr)
	}
	return err
}
