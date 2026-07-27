package telerr_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/telerr"
)

func TestOf_ReportsKindOfDomainError(t *testing.T) {
	err := &telerr.Error{Kind: telerr.RateLimited, Op: "messages.sendMessage"}
	assert.Equal(t, telerr.RateLimited, telerr.Of(err))
}

func TestOf_WalksWrappedChain(t *testing.T) {
	inner := &telerr.Error{Kind: telerr.Forbidden}
	wrapped := fmt.Errorf("forward messages: %w", inner)
	assert.Equal(t, telerr.Forbidden, telerr.Of(wrapped))
}

func TestOf_PlainErrorIsInternal(t *testing.T) {
	assert.Equal(t, telerr.Internal, telerr.Of(errors.New("disk full")))
}

func TestOf_NilIsEmpty(t *testing.T) {
	assert.Equal(t, telerr.Kind(""), telerr.Of(nil))
}

func TestAs_ExposesFields(t *testing.T) {
	inner := &telerr.Error{Kind: telerr.RateLimited, RetryAfter: 30 * time.Second}
	e, ok := telerr.As(fmt.Errorf("send: %w", inner))
	require.True(t, ok)
	assert.Equal(t, 30*time.Second, e.RetryAfter)
}

func TestAs_PlainErrorIsNotDomain(t *testing.T) {
	_, ok := telerr.As(errors.New("disk full"))
	assert.False(t, ok)
}

func TestError_UnwrapReturnsCause(t *testing.T) {
	cause := errors.New("original")
	err := &telerr.Error{Kind: telerr.Network, Cause: cause}
	assert.True(t, errors.Is(err, cause))
}

func TestError_MessageNamesOpAndDetail(t *testing.T) {
	err := &telerr.Error{Kind: telerr.Forbidden, Op: "messages.forwardMessages", Detail: "CHAT_FORWARDS_RESTRICTED"}
	assert.Equal(t, "messages.forwardMessages: forbidden (CHAT_FORWARDS_RESTRICTED)", err.Error())
}

func TestError_MessageWithoutDetail(t *testing.T) {
	err := &telerr.Error{Kind: telerr.Network, Op: "acquire api"}
	assert.Equal(t, "acquire api: network", err.Error())
}
