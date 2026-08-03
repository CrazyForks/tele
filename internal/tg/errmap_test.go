package tg

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gotd/td/bin"
	"github.com/gotd/td/rpc"
	gotdtg "github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/sorokin-vladimir/tele/internal/telerr"
)

func testClient() *GotdClient {
	return &GotdClient{log: zap.NewNop(), traceLog: zap.NewNop()}
}

func TestClassifyTgErr(t *testing.T) {
	tests := []struct {
		name     string
		err      *tgerr.Error
		wantKind telerr.Kind
		wantWait time.Duration
	}{
		{"flood wait", &tgerr.Error{Code: 420, Type: "FLOOD_WAIT", Argument: 30}, telerr.RateLimited, 30 * time.Second},
		{"slowmode wait", &tgerr.Error{Code: 420, Type: "SLOWMODE_WAIT", Argument: 5}, telerr.RateLimited, 5 * time.Second},
		{"auth key unregistered", &tgerr.Error{Code: 401, Type: "AUTH_KEY_UNREGISTERED"}, telerr.Unauthorized, 0},
		{"auth key duplicated", &tgerr.Error{Code: 406, Type: "AUTH_KEY_DUPLICATED"}, telerr.Unauthorized, 0},
		{"forbidden by code", &tgerr.Error{Code: 403, Type: "CHAT_WRITE_FORBIDDEN"}, telerr.Forbidden, 0},
		{"peer id invalid", &tgerr.Error{Code: 400, Type: "PEER_ID_INVALID"}, telerr.PeerNotFound, 0},
		{"channel invalid", &tgerr.Error{Code: 400, Type: "CHANNEL_INVALID"}, telerr.PeerNotFound, 0},
		{"forwards restricted", &tgerr.Error{Code: 400, Type: "CHAT_FORWARDS_RESTRICTED"}, telerr.Forbidden, 0},
		{"send media forbidden by suffix", &tgerr.Error{Code: 400, Type: "CHAT_SEND_MEDIA_FORBIDDEN"}, telerr.Forbidden, 0},
		{"file reference expired", &tgerr.Error{Code: 400, Type: "FILE_REFERENCE_EXPIRED"}, telerr.StaleReference, 0},
		{"message id invalid", &tgerr.Error{Code: 400, Type: "MESSAGE_ID_INVALID"}, telerr.NotFound, 0},
		{"server error", &tgerr.Error{Code: 500, Type: "INTERNAL"}, telerr.Network, 0},
		{"unmapped", &tgerr.Error{Code: 400, Type: "SOMETHING_NEW"}, telerr.Internal, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, wait := classifyTgErr(tt.err)
			assert.Equal(t, tt.wantKind, kind)
			assert.Equal(t, tt.wantWait, wait)
		})
	}
}

func TestMapError_NilStaysNil(t *testing.T) {
	assert.NoError(t, testClient().mapError("messages.sendMessage", nil))
}

func TestMapError_SetsOpAndDetail(t *testing.T) {
	err := testClient().mapError("messages.sendMessage", &tgerr.Error{Code: 400, Type: "PEER_ID_INVALID"})
	e, ok := telerr.As(err)
	require.True(t, ok)
	assert.Equal(t, telerr.PeerNotFound, e.Kind)
	assert.Equal(t, "messages.sendMessage", e.Op)
	assert.Equal(t, "PEER_ID_INVALID", e.Detail)
}

func TestMapError_ServerErrorIsTransient(t *testing.T) {
	err := testClient().mapError("messages.getHistory", &tgerr.Error{Code: 500, Type: "INTERNAL"})
	e, ok := telerr.As(err)
	require.True(t, ok)
	assert.True(t, e.Transient)
}

func TestMapError_NetErrorIsTransientNetwork(t *testing.T) {
	var netErr net.Error = &net.DNSError{Err: "no such host", IsTemporary: true}
	err := testClient().mapError("messages.getHistory", netErr)
	e, ok := telerr.As(err)
	require.True(t, ok)
	assert.Equal(t, telerr.Network, e.Kind)
	assert.True(t, e.Transient)
}

func TestMapError_ContextErrorsPassThrough(t *testing.T) {
	c := testClient()
	for _, ctxErr := range []error{context.Canceled, context.DeadlineExceeded} {
		mapped := c.mapError("op", ctxErr)
		assert.ErrorIs(t, mapped, ctxErr)
		_, isDomain := telerr.As(mapped)
		assert.False(t, isDomain, "cancellation is control flow, not a domain failure")
	}
}

func TestMapError_AlreadyMappedPassesThrough(t *testing.T) {
	already := &telerr.Error{Kind: telerr.NotFound, Op: "first"}
	assert.Same(t, error(already), testClient().mapError("second", already))
}

func TestMapError_PlainErrorIsInternalWithText(t *testing.T) {
	err := testClient().mapError("op", errors.New("boom"))
	e, ok := telerr.As(err)
	require.True(t, ok)
	assert.Equal(t, telerr.Internal, e.Kind)
	assert.Equal(t, "boom", e.Detail)
}

// fakeInvoker stands in for the gotd RPC chain.
type fakeInvoker struct{ err error }

func (f *fakeInvoker) Invoke(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
	return f.err
}

func TestErrorMiddleware_MapsAndNamesTheOperation(t *testing.T) {
	c := testClient()
	invoker := c.errorMiddleware().Handle(&fakeInvoker{err: &tgerr.Error{Code: 400, Type: "PEER_ID_INVALID"}})

	err := invoker.Invoke(context.Background(), &gotdtg.MessagesSendMessageRequest{}, nil)

	e, ok := telerr.As(err)
	require.True(t, ok)
	assert.Equal(t, telerr.PeerNotFound, e.Kind)
	assert.Equal(t, "messages.sendMessage", e.Op)
}

func TestErrorMiddleware_SuccessStaysNil(t *testing.T) {
	c := testClient()
	invoker := c.errorMiddleware().Handle(&fakeInvoker{})
	assert.NoError(t, invoker.Invoke(context.Background(), &gotdtg.MessagesSendMessageRequest{}, nil))
}

func TestAcquireAPI_NotConnectedIsTransientNetwork(t *testing.T) {
	_, err := testClient().acquireAPI()
	e, ok := telerr.As(err)
	require.True(t, ok)
	assert.Equal(t, telerr.Network, e.Kind)
	assert.True(t, e.Transient)
}

// The invariant that keeps gotd working: it recognises its own errors through
// the unwrap chain, so mapping must not hide them.
func TestMapError_PreservesTgerrForGotd(t *testing.T) {
	mapped := testClient().mapError("op", &tgerr.Error{Code: 420, Type: "FLOOD_WAIT", Argument: 3})
	assert.True(t, tgerr.Is(mapped, "FLOOD_WAIT"))

	var te *tgerr.Error
	require.True(t, errors.As(mapped, &te))
	assert.Equal(t, 420, te.Code)
}

// gotd reports a request the server never acknowledged as RetryLimitReachedErr.
// Its own documentation calls it exactly that — "server does not acknowledge
// request after multiple retries" — which is a transient transport failure, not
// a bug. Left unmapped it became Internal, which is terminal: an offline send
// was marked failed forever instead of waiting for the connection (#193).
func TestMapError_UnacknowledgedRequestIsTransientNetwork(t *testing.T) {
	c := testClient()

	got := c.mapError("messages.sendMessage", &rpc.RetryLimitReachedErr{Retries: 5})

	e, ok := telerr.As(got)
	require.True(t, ok)
	assert.Equal(t, telerr.Network, e.Kind)
	assert.True(t, e.Transient, "an unacknowledged request is worth repeating")
}

func TestMapError_AClosedEngineIsTransientNetwork(t *testing.T) {
	c := testClient()

	got := c.mapError("messages.sendMessage", rpc.ErrEngineClosed)

	e, ok := telerr.As(got)
	require.True(t, ok)
	assert.Equal(t, telerr.Network, e.Kind)
	assert.True(t, e.Transient)
}

// Wrapped the way gotd actually delivers it, through rpcDoRequest.
func TestMapError_UnacknowledgedRequestThroughAWrapChain(t *testing.T) {
	c := testClient()
	wrapped := fmt.Errorf("rpcDoRequest: %w", &rpc.RetryLimitReachedErr{Retries: 5})

	got := c.mapError("messages.sendMessage", wrapped)

	assert.Equal(t, telerr.Network, telerr.Of(got))
}
