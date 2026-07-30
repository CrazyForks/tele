package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sorokin-vladimir/tele/internal/domain"
)

// The names are part of the owner's vocabulary: they go into logs now and onto
// the wire in v2, so they must be stable and readable.
func TestMediaSlot_NamesEachSlot(t *testing.T) {
	assert.Equal(t, "photo_thumb", domain.PhotoThumb.String())
	assert.Equal(t, "photo_full", domain.PhotoFull.String())
	assert.Equal(t, "doc_thumb", domain.DocThumb.String())
	assert.Equal(t, "doc_full", domain.DocFull.String())
}

func TestMediaSlot_NamesAnUnknownSlot(t *testing.T) {
	assert.Equal(t, "unknown", domain.MediaSlot(99).String())
}
