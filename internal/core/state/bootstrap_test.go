package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/store"
)

func TestSetDialogs_WritesEveryChat(t *testing.T) {
	s, st := newState(t)

	s.SetDialogs([]store.Chat{
		{ID: 1, Title: "A", UnreadCount: 2},
		{ID: 2, Title: "B"},
	})

	c, ok := st.GetChat(1)
	require.True(t, ok)
	assert.Equal(t, 2, c.UnreadCount)
	_, ok = st.GetChat(2)
	assert.True(t, ok)
}

func TestSetFolderFilters_WritesFilters(t *testing.T) {
	s, st := newState(t)

	s.SetFolderFilters([]store.FolderFilter{{ID: 7, Title: "Work"}})

	got := st.FolderFilters()
	require.Len(t, got, 1)
	assert.Equal(t, 7, got[0].ID)
}
