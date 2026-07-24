package components

import (
	"strings"
	"testing"
	"time"

	"github.com/sorokin-vladimir/tele/internal/store"
)

func mustDate(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func ids(groups [][]store.Message) [][]int {
	out := make([][]int, len(groups))
	for i, g := range groups {
		for _, m := range g {
			out[i] = append(out[i], m.ID)
		}
	}
	return out
}

func TestGroupParts(t *testing.T) {
	cases := []struct {
		name string
		in   []store.Message
		want [][]int
	}{
		{
			name: "no albums: each message is its own group",
			in: []store.Message{
				{ID: 1, SenderID: 7},
				{ID: 2, SenderID: 7},
			},
			want: [][]int{{1}, {2}},
		},
		{
			name: "contiguous same-sender album coalesces",
			in: []store.Message{
				{ID: 1, SenderID: 7, GroupedID: 100},
				{ID: 2, SenderID: 7, GroupedID: 100},
				{ID: 3, SenderID: 7, GroupedID: 100},
			},
			want: [][]int{{1, 2, 3}},
		},
		{
			name: "album bounded by a plain message",
			in: []store.Message{
				{ID: 1, SenderID: 7, GroupedID: 100},
				{ID: 2, SenderID: 7, GroupedID: 100},
				{ID: 3, SenderID: 7},
			},
			want: [][]int{{1, 2}, {3}},
		},
		{
			name: "different grouped_id does not merge",
			in: []store.Message{
				{ID: 1, SenderID: 7, GroupedID: 100},
				{ID: 2, SenderID: 7, GroupedID: 200},
			},
			want: [][]int{{1}, {2}},
		},
		{
			name: "same grouped_id but different sender does not merge",
			in: []store.Message{
				{ID: 1, SenderID: 7, GroupedID: 100},
				{ID: 2, SenderID: 8, GroupedID: 100},
			},
			want: [][]int{{1}, {2}},
		},
		{
			name: "single remaining part renders as a normal group of one",
			in: []store.Message{
				{ID: 1, SenderID: 7, GroupedID: 100},
			},
			want: [][]int{{1}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ids(groupParts(tc.in))
			if len(got) != len(tc.want) {
				t.Fatalf("groups = %v, want %v", got, tc.want)
			}
			for i := range got {
				if len(got[i]) != len(tc.want[i]) {
					t.Fatalf("group %d = %v, want %v", i, got[i], tc.want[i])
				}
				for j := range got[i] {
					if got[i][j] != tc.want[i][j] {
						t.Fatalf("group %d = %v, want %v", i, got[i], tc.want[i])
					}
				}
			}
		})
	}
}

func TestBuildItemsGroupsAlbum(t *testing.T) {
	ml := NewMessageList(20, 40)
	ml.SetMessages([]store.Message{
		{ID: 1, SenderID: 7, GroupedID: 100, Date: mustDate("2026-07-24T10:00:00Z")},
		{ID: 2, SenderID: 7, GroupedID: 100, Date: mustDate("2026-07-24T10:00:00Z")},
		{ID: 3, SenderID: 7, GroupedID: 100, Date: mustDate("2026-07-24T10:00:00Z")},
	})
	if got := ml.Count(); got != 1 {
		t.Fatalf("Count() = %d, want 1 (album collapses to one item)", got)
	}
	for _, id := range []int{1, 2, 3} {
		if ml.findMessage(id) == nil {
			t.Fatalf("findMessage(%d) = nil, want the album part", id)
		}
	}
}

func TestGroupMediaParts(t *testing.T) {
	parts := []store.Message{
		{ID: 1, Photo: &store.PhotoRef{ID: 11}},
		{ID: 2, Text: "no media"},
		{ID: 3, Photo: &store.PhotoRef{ID: 33}},
	}
	got := groupMediaParts(parts)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (text-only part excluded)", len(got))
	}
	if got[0].Index != 1 || got[1].Index != 2 {
		t.Fatalf("indexes = %d,%d, want 1,2", got[0].Index, got[1].Index)
	}
	if got[0].Msg.ID != 1 || got[1].Msg.ID != 3 {
		t.Fatalf("ids = %d,%d, want 1,3", got[0].Msg.ID, got[1].Msg.ID)
	}
}

func TestAlbumImageRowsScalesDown(t *testing.T) {
	ml := NewMessageList(24, 60) // viewHeight 24
	photo := func(id int64) store.Message {
		return store.Message{ID: int(id), GroupedID: 100, Photo: &store.PhotoRef{ID: id}}
	}
	one := ml.albumImageRows([]store.Message{photo(1)})
	many := ml.albumImageRows([]store.Message{photo(1), photo(2), photo(3), photo(4)})
	if many >= one {
		t.Fatalf("per-part rows did not shrink: one=%d many=%d", one, many)
	}
	if many < 2 {
		t.Fatalf("per-part rows = %d, want >= 2 (readability floor)", many)
	}
}

func TestGroupHeightBoundedByBudget(t *testing.T) {
	ml := NewMessageList(24, 60)
	parts := []store.Message{
		{ID: 1, GroupedID: 100, Photo: &store.PhotoRef{ID: 1}},
		{ID: 2, GroupedID: 100, Photo: &store.PhotoRef{ID: 2}},
		{ID: 3, GroupedID: 100, Photo: &store.PhotoRef{ID: 3}},
		{ID: 4, GroupedID: 100, Photo: &store.PhotoRef{ID: 4}, Text: "album caption"},
	}
	h := ml.groupHeight(parts)
	if h <= 0 {
		t.Fatalf("groupHeight = %d, want > 0", h)
	}
	if h > ml.viewHeight {
		t.Fatalf("groupHeight = %d exceeds viewHeight %d; scale-down failed", h, ml.viewHeight)
	}
}

func TestRenderGroupBubbleShowsBadgesAndCaption(t *testing.T) {
	ml := NewMessageList(24, 60)
	parts := []store.Message{
		{ID: 1, GroupedID: 100, SenderID: 7, Photo: &store.PhotoRef{ID: 1}},
		{ID: 2, GroupedID: 100, SenderID: 7, Photo: &store.PhotoRef{ID: 2}, Text: "hi album"},
	}
	out := strings.Join(ml.renderGroupBubble(parts, false), "\n")
	if !strings.Contains(out, "[1]") || !strings.Contains(out, "[2]") {
		t.Fatalf("missing index badges in:\n%s", out)
	}
	if !strings.Contains(out, "hi album") {
		t.Fatalf("missing shared caption in:\n%s", out)
	}
}

func TestGroupHeightMatchesRender(t *testing.T) {
	ml := NewMessageList(24, 60)
	parts := []store.Message{
		{ID: 1, GroupedID: 100, SenderID: 7, Photo: &store.PhotoRef{ID: 1}},
		{ID: 2, GroupedID: 100, SenderID: 7, Photo: &store.PhotoRef{ID: 2}},
		{ID: 3, GroupedID: 100, SenderID: 7, Photo: &store.PhotoRef{ID: 3}, Text: "cap"},
	}
	want := ml.groupHeight(parts)
	got := len(ml.renderGroupBubble(parts, false))
	if got != want {
		t.Fatalf("render lines = %d, groupHeight = %d; they must match", got, want)
	}
}

func TestRenderGroupBubbleFileRows(t *testing.T) {
	ml := NewMessageList(24, 70)
	parts := []store.Message{
		{ID: 1, GroupedID: 100, SenderID: 7,
			Media:    &store.MediaRef{Kind: store.MediaFile, FileName: "report.pdf", Size: 2048},
			Document: &store.DocumentRef{ID: 1, FileName: "report.pdf"}},
		{ID: 2, GroupedID: 100, SenderID: 7,
			Media:    &store.MediaRef{Kind: store.MediaFile, FileName: "data.csv", Size: 4096},
			Document: &store.DocumentRef{ID: 2, FileName: "data.csv"}},
	}
	out := strings.Join(ml.renderGroupBubble(parts, false), "\n")
	if !strings.Contains(out, "report.pdf") || !strings.Contains(out, "data.csv") {
		t.Fatalf("file names missing in file-row album:\n%s", out)
	}
	if !strings.Contains(out, "[1]") || !strings.Contains(out, "[2]") {
		t.Fatalf("badges missing in file-row album:\n%s", out)
	}
}

func TestRenderGroupBubbleBadgeShowsTypeAndContext(t *testing.T) {
	ml := NewMessageList(24, 70)
	parts := []store.Message{
		{ID: 1, GroupedID: 100, SenderID: 7, Photo: &store.PhotoRef{ID: 1}},
		{ID: 2, GroupedID: 100, SenderID: 7,
			Media:    &store.MediaRef{Kind: store.MediaVideo, Duration: 34},
			Document: &store.DocumentRef{ID: 2}},
	}
	out := strings.Join(ml.renderGroupBubble(parts, false), "\n")
	if !strings.Contains(out, "[1] 📷 photo") {
		t.Fatalf("photo badge missing type/context in:\n%s", out)
	}
	if !strings.Contains(out, "[2] 🎥 video 0:34") {
		t.Fatalf("video badge missing duration context in:\n%s", out)
	}
}

func TestRenderGroupBubbleBlankLineBetweenItems(t *testing.T) {
	ml := NewMessageList(24, 60)
	parts := []store.Message{
		{ID: 1, GroupedID: 100, SenderID: 7, Photo: &store.PhotoRef{ID: 1}},
		{ID: 2, GroupedID: 100, SenderID: 7, Photo: &store.PhotoRef{ID: 2}},
	}
	// A two-part album with no caption must reserve exactly one more line than the
	// same album rendered without an inter-item separator would: badges + previews
	// + one blank between them + two borders.
	rows := ml.albumImageRows(parts)
	want := 2 /*borders*/ + 2*(1 /*badge*/ +rows) + 1 /*inter-item blank*/
	if got := len(ml.renderGroupBubble(parts, false)); got != want {
		t.Fatalf("album line count = %d, want %d (missing inter-item blank?)", got, want)
	}
}
