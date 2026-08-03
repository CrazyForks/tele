package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublishProgress_DeliversToTheChannel(t *testing.T) {
	o, _ := newCmdOwner(t, &stubClient{})

	o.publishProgress(Progress{ChatID: 1, Ref: "r1#0", Part: 2, Parts: 5, Done: 30, Total: 100})

	select {
	case got := <-o.Progress():
		assert.Equal(t, "r1#0", got.Ref)
		assert.Equal(t, 2, got.Part)
		assert.Equal(t, int64(30), got.Done)
	default:
		require.Fail(t, "nothing was published")
	}
}

func TestPublishProgress_DropsWhenNobodyIsDraining(t *testing.T) {
	o, _ := newCmdOwner(t, &stubClient{})

	// More frames than the buffer holds. A dropped progress frame costs one
	// repaint; blocking the uploader would cost the upload.
	for i := 0; i < 1000; i++ {
		o.publishProgress(Progress{ChatID: 1, Ref: "r1#0", Done: int64(i)})
	}

	assert.NotEmpty(t, o.Progress(), "the channel should still hold what it could")
}
