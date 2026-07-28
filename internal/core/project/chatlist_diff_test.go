package project_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/core/project"
)

func contents(rows ...project.ChatRow) project.ChatListContents {
	return project.ChatListContents{
		Offset:  0,
		Total:   len(rows),
		Rows:    rows,
		Folders: project.FolderCounts{Unread: map[int]int{}},
	}
}

func TestDiffChatList_IdenticalContentsProduceNothing(t *testing.T) {
	c := contents(project.ChatRow{ID: 1}, project.ChatRow{ID: 2})

	assert.Empty(t, project.DiffChatList(c, c),
		"a change outside the window must cost nothing — this is the whole point")
}

func TestDiffChatList_SameOrderChangedRowProducesOneRowDelta(t *testing.T) {
	prev := contents(project.ChatRow{ID: 1}, project.ChatRow{ID: 2})
	next := contents(project.ChatRow{ID: 1}, project.ChatRow{ID: 2, Online: true})

	got := project.DiffChatList(prev, next)

	require.Len(t, got, 1)
	assert.Equal(t, project.ChatListRow, got[0].Kind)
	assert.Equal(t, int64(2), got[0].Row.ID)
	assert.True(t, got[0].Row.Online)
}

func TestDiffChatList_TwoChangedRowsProduceTwoRowDeltas(t *testing.T) {
	prev := contents(project.ChatRow{ID: 1}, project.ChatRow{ID: 2})
	next := contents(project.ChatRow{ID: 1, Unread: 1}, project.ChatRow{ID: 2, Muted: true})

	got := project.DiffChatList(prev, next)

	require.Len(t, got, 2)
	assert.Equal(t, int64(1), got[0].Row.ID)
	assert.Equal(t, int64(2), got[1].Row.ID)
}

func TestDiffChatList_ReorderProducesReset(t *testing.T) {
	prev := contents(project.ChatRow{ID: 1}, project.ChatRow{ID: 2})
	next := contents(project.ChatRow{ID: 2}, project.ChatRow{ID: 1})

	got := project.DiffChatList(prev, next)

	require.Len(t, got, 1)
	assert.Equal(t, project.ChatListReset, got[0].Kind,
		"a bumped chat moves every row below it; shift algebra is where off-by-one lives")
	assert.Equal(t, next.Rows, got[0].Rows)
	assert.Equal(t, next.Total, got[0].Total)
}

func TestDiffChatList_MembershipChangeProducesReset(t *testing.T) {
	prev := contents(project.ChatRow{ID: 1}, project.ChatRow{ID: 2})
	next := contents(project.ChatRow{ID: 1}, project.ChatRow{ID: 3})

	got := project.DiffChatList(prev, next)

	require.Len(t, got, 1)
	assert.Equal(t, project.ChatListReset, got[0].Kind)
}

func TestDiffChatList_TotalChangeAloneProducesReset(t *testing.T) {
	prev := contents(project.ChatRow{ID: 1})
	next := contents(project.ChatRow{ID: 1})
	next.Total = 42 // a chat entered the list below the window

	got := project.DiffChatList(prev, next)

	require.Len(t, got, 1)
	assert.Equal(t, project.ChatListReset, got[0].Kind, "the scrollbar depends on Total")
}

func TestDiffChatList_OffsetChangeProducesReset(t *testing.T) {
	prev := contents(project.ChatRow{ID: 1})
	next := contents(project.ChatRow{ID: 1})
	next.Offset = 5

	got := project.DiffChatList(prev, next)

	require.Len(t, got, 1)
	assert.Equal(t, project.ChatListReset, got[0].Kind)
}

func TestDiffChatList_FolderCountChangeProducesFoldersDeltaOnly(t *testing.T) {
	prev := contents(project.ChatRow{ID: 1})
	next := contents(project.ChatRow{ID: 1})
	next.Folders = project.FolderCounts{Unread: map[int]int{7: 3}}

	got := project.DiffChatList(prev, next)

	require.Len(t, got, 1)
	assert.Equal(t, project.ChatListFolders, got[0].Kind,
		"an unread change outside the window still moves a folder badge")
	assert.Equal(t, 3, got[0].Folders.Unread[7])
}

func TestDiffChatList_ArchivePresenceChangeProducesFoldersDelta(t *testing.T) {
	prev := contents(project.ChatRow{ID: 1})
	next := contents(project.ChatRow{ID: 1})
	next.Folders = project.FolderCounts{Unread: map[int]int{}, ArchivePresent: true}

	got := project.DiffChatList(prev, next)

	require.Len(t, got, 1)
	assert.Equal(t, project.ChatListFolders, got[0].Kind)
	assert.True(t, got[0].Folders.ArchivePresent)
}

func TestDiffChatList_ResetAndFoldersTogether(t *testing.T) {
	prev := contents(project.ChatRow{ID: 1}, project.ChatRow{ID: 2})
	next := contents(project.ChatRow{ID: 2}, project.ChatRow{ID: 1})
	next.Folders = project.FolderCounts{Unread: map[int]int{7: 1}}

	got := project.DiffChatList(prev, next)

	require.Len(t, got, 2)
	assert.Equal(t, project.ChatListReset, got[0].Kind)
	assert.Equal(t, project.ChatListFolders, got[1].Kind)
}

func TestDiffChatList_FirstBuildAgainstZeroContentsIsAReset(t *testing.T) {
	got := project.DiffChatList(project.ChatListContents{}, contents(project.ChatRow{ID: 1}))

	require.NotEmpty(t, got)
	assert.Equal(t, project.ChatListReset, got[0].Kind,
		"a subscription's first delta is always a Reset, so resubscribing is resync")
	assert.Len(t, got, 1, "an empty and a nil folder map are the same folder state")
}

func TestDiffChatList_EmptyWindowStaysEmpty(t *testing.T) {
	assert.Empty(t, project.DiffChatList(contents(), contents()))
}
