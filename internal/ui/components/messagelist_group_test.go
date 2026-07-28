package components

import (
	"image"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/sorokin-vladimir/tele/internal/domain"
)

func mustDate(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func ids(groups [][]domain.Message) [][]int {
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
		in   []domain.Message
		want [][]int
	}{
		{
			name: "no albums: each message is its own group",
			in: []domain.Message{
				{ID: 1, SenderID: 7},
				{ID: 2, SenderID: 7},
			},
			want: [][]int{{1}, {2}},
		},
		{
			name: "contiguous same-sender album coalesces",
			in: []domain.Message{
				{ID: 1, SenderID: 7, GroupedID: 100},
				{ID: 2, SenderID: 7, GroupedID: 100},
				{ID: 3, SenderID: 7, GroupedID: 100},
			},
			want: [][]int{{1, 2, 3}},
		},
		{
			name: "album bounded by a plain message",
			in: []domain.Message{
				{ID: 1, SenderID: 7, GroupedID: 100},
				{ID: 2, SenderID: 7, GroupedID: 100},
				{ID: 3, SenderID: 7},
			},
			want: [][]int{{1, 2}, {3}},
		},
		{
			name: "different grouped_id does not merge",
			in: []domain.Message{
				{ID: 1, SenderID: 7, GroupedID: 100},
				{ID: 2, SenderID: 7, GroupedID: 200},
			},
			want: [][]int{{1}, {2}},
		},
		{
			name: "same grouped_id but different sender does not merge",
			in: []domain.Message{
				{ID: 1, SenderID: 7, GroupedID: 100},
				{ID: 2, SenderID: 8, GroupedID: 100},
			},
			want: [][]int{{1}, {2}},
		},
		{
			name: "single remaining part renders as a normal group of one",
			in: []domain.Message{
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
	ml.SetMessages([]domain.Message{
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
	parts := []domain.Message{
		{ID: 1, Photo: &domain.PhotoRef{ID: 11}},
		{ID: 2, Text: "no media"},
		{ID: 3, Photo: &domain.PhotoRef{ID: 33}},
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
	photo := func(id int64) domain.Message {
		return domain.Message{ID: int(id), GroupedID: 100, Photo: &domain.PhotoRef{ID: id}}
	}
	one := ml.albumImageRows([]domain.Message{photo(1)})
	many := ml.albumImageRows([]domain.Message{photo(1), photo(2), photo(3), photo(4)})
	if many >= one {
		t.Fatalf("per-part rows did not shrink: one=%d many=%d", one, many)
	}
	if many < 2 {
		t.Fatalf("per-part rows = %d, want >= 2 (readability floor)", many)
	}
}

func TestGroupHeightBoundedByBudget(t *testing.T) {
	ml := NewMessageList(24, 60)
	// Tall (portrait) images so each preview hits the per-part row budget: the
	// loaded album must fill the pane without exceeding it (folded badges reclaim
	// the rows a separate badge line would have cost).
	for _, id := range []int64{1, 2, 3, 4} {
		ml.SetImage(id, image.NewRGBA(image.Rect(0, 0, 600, 800))) // 3:4 portrait
	}
	parts := []domain.Message{
		{ID: 1, GroupedID: 100, Media: &domain.MediaRef{Kind: domain.MediaPhoto}, Photo: &domain.PhotoRef{ID: 1}},
		{ID: 2, GroupedID: 100, Media: &domain.MediaRef{Kind: domain.MediaPhoto}, Photo: &domain.PhotoRef{ID: 2}},
		{ID: 3, GroupedID: 100, Media: &domain.MediaRef{Kind: domain.MediaPhoto}, Photo: &domain.PhotoRef{ID: 3}},
		{ID: 4, GroupedID: 100, Media: &domain.MediaRef{Kind: domain.MediaPhoto}, Photo: &domain.PhotoRef{ID: 4}, Text: "album caption"},
	}
	h := ml.groupHeight(parts)
	if h > ml.viewHeight {
		t.Fatalf("loaded album height = %d exceeds viewHeight %d; scale-down failed", h, ml.viewHeight)
	}
	if h < ml.viewHeight-len(parts) {
		t.Fatalf("loaded album height = %d falls short of viewHeight %d; badges not reclaimed", h, ml.viewHeight)
	}
}

func TestRenderGroupBubbleShowsBadgesAndCaption(t *testing.T) {
	ml := NewMessageList(24, 60)
	parts := []domain.Message{
		{ID: 1, GroupedID: 100, SenderID: 7, Photo: &domain.PhotoRef{ID: 1}},
		{ID: 2, GroupedID: 100, SenderID: 7, Photo: &domain.PhotoRef{ID: 2}, Text: "hi album"},
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
	parts := []domain.Message{
		{ID: 1, GroupedID: 100, SenderID: 7, Photo: &domain.PhotoRef{ID: 1}},
		{ID: 2, GroupedID: 100, SenderID: 7, Photo: &domain.PhotoRef{ID: 2}},
		{ID: 3, GroupedID: 100, SenderID: 7, Photo: &domain.PhotoRef{ID: 3}, Text: "cap"},
	}
	want := ml.groupHeight(parts)
	got := len(ml.renderGroupBubble(parts, false))
	if got != want {
		t.Fatalf("render lines = %d, groupHeight = %d; they must match", got, want)
	}
}

func TestRenderGroupBubbleFileRows(t *testing.T) {
	ml := NewMessageList(24, 70)
	parts := []domain.Message{
		{ID: 1, GroupedID: 100, SenderID: 7,
			Media:    &domain.MediaRef{Kind: domain.MediaFile, FileName: "report.pdf", Size: 2048},
			Document: &domain.DocumentRef{ID: 1, FileName: "report.pdf"}},
		{ID: 2, GroupedID: 100, SenderID: 7,
			Media:    &domain.MediaRef{Kind: domain.MediaFile, FileName: "data.csv", Size: 4096},
			Document: &domain.DocumentRef{ID: 2, FileName: "data.csv"}},
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
	parts := []domain.Message{
		{ID: 1, GroupedID: 100, SenderID: 7, Photo: &domain.PhotoRef{ID: 1}},
		{ID: 2, GroupedID: 100, SenderID: 7,
			Media:    &domain.MediaRef{Kind: domain.MediaVideo, Duration: 34},
			Document: &domain.DocumentRef{ID: 2}},
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
	parts := []domain.Message{
		{ID: 1, GroupedID: 100, SenderID: 7, Photo: &domain.PhotoRef{ID: 1}},
		{ID: 2, GroupedID: 100, SenderID: 7, Photo: &domain.PhotoRef{ID: 2}},
	}
	// A two-part album with no caption must reserve exactly one more line than the
	// same album rendered without an inter-item separator would: badges + previews
	// + one blank between them + two borders.
	rows := ml.albumImageRows(parts)
	want := 2 /*borders*/ + 2*(1 /*badge*/ +rows) + 1 /*inter-item blank*/
	// This asserts the vertical-stack layout specifically (the grid fallback).
	if got := len(ml.renderGroupStack(parts, false)); got != want {
		t.Fatalf("album line count = %d, want %d (missing inter-item blank?)", got, want)
	}
}

func TestAlbumPreviewDownscalesNotCrops(t *testing.T) {
	ml := NewMessageList(24, 80)
	// A very tall (portrait) image: cropping to the budget would keep only its top;
	// downscaling must fit the whole image, which forces the width narrower.
	tall := image.NewRGBA(image.Rect(0, 0, 400, 2000))
	ml.SetImage(11, tall)

	parts := []domain.Message{
		{ID: 1, GroupedID: 100, SenderID: 7,
			Media: &domain.MediaRef{Kind: domain.MediaPhoto}, Photo: &domain.PhotoRef{ID: 11}},
		{ID: 2, GroupedID: 100, SenderID: 7,
			Media: &domain.MediaRef{Kind: domain.MediaPhoto}, Photo: &domain.PhotoRef{ID: 12}},
	}

	budget := ml.albumImageRows(parts)
	cols, rows := ml.albumPartBox(budget, 400, 2000)
	if rows > budget {
		t.Fatalf("downscaled rows = %d exceed budget %d (still cropping?)", rows, budget)
	}
	if cols >= ml.photoContentCols() {
		t.Fatalf("tall image not narrowed: cols=%d, contentCols=%d", cols, ml.photoContentCols())
	}
	// Render and height stay in lock-step with a real cached image.
	if got, want := len(ml.renderGroupBubble(parts, false)), ml.groupHeight(parts); got != want {
		t.Fatalf("render lines = %d, groupHeight = %d; must match with a cached image", got, want)
	}
}

func TestAlbumPhotoBoxStableWhenSiblingVideoLoads(t *testing.T) {
	ml := NewMessageList(24, 80)
	ml.SetImage(11, image.NewRGBA(image.Rect(0, 0, 400, 300))) // photo already cached
	parts := []domain.Message{
		{ID: 1, GroupedID: 100, SenderID: 7, Media: &domain.MediaRef{Kind: domain.MediaPhoto}, Photo: &domain.PhotoRef{ID: 11}},
		{ID: 2, GroupedID: 100, SenderID: 7, Media: &domain.MediaRef{Kind: domain.MediaVideo}, Document: &domain.DocumentRef{ID: 22, ThumbSize: "m"}},
		{ID: 3, GroupedID: 100, SenderID: 7, Media: &domain.MediaRef{Kind: domain.MediaPhoto}, Photo: &domain.PhotoRef{ID: 33}},
	}
	ml.SetMessages(parts)

	c1, r1 := ml.MediaBoxForID(11, 400, 300)
	// The video thumbnail arrives later (first open after restart).
	ml.SetImage(22, image.NewRGBA(image.Rect(0, 0, 320, 240)))
	c2, r2 := ml.MediaBoxForID(11, 400, 300)

	if c1 != c2 || r1 != r2 {
		t.Fatalf("photo box changed when sibling video loaded: (%d,%d) -> (%d,%d); "+
			"placement no longer matches render, photo disappears", c1, r1, c2, r2)
	}
}

func TestAlbumBadgeFoldedOntoCachedArt(t *testing.T) {
	ml := NewMessageList(24, 80)
	ml.SetImage(11, image.NewRGBA(image.Rect(0, 0, 400, 300)))
	ml.SetImage(22, image.NewRGBA(image.Rect(0, 0, 400, 300)))
	parts := []domain.Message{
		{ID: 1, GroupedID: 100, SenderID: 7, Media: &domain.MediaRef{Kind: domain.MediaPhoto}, Photo: &domain.PhotoRef{ID: 11}},
		{ID: 2, GroupedID: 100, SenderID: 7, Media: &domain.MediaRef{Kind: domain.MediaPhoto}, Photo: &domain.PhotoRef{ID: 22}},
	}
	// Vertical-stack layout specifically (the grid fallback): badge folds onto row 0.
	out := ml.renderGroupStack(parts, false)
	joined := strings.Join(out, "\n")
	if !strings.Contains(joined, "[1]") || !strings.Contains(joined, "[2]") {
		t.Fatalf("badges missing from folded art:\n%s", joined)
	}
	// The badge is folded onto the image's first row, so it must NOT add a separate
	// row: total lines are borders + each part's art rows + one inter-item blank.
	budget := ml.albumImageRows(parts)
	r1 := ml.albumPartRows(budget, parts[0])
	r2 := ml.albumPartRows(budget, parts[1])
	want := 2 + r1 + r2 + 1
	if len(out) != want {
		t.Fatalf("folded album lines = %d, want %d (badge must not add a row)", len(out), want)
	}
	if got := ml.groupHeightStack(parts); got != len(out) {
		t.Fatalf("groupHeightStack = %d, render = %d; must match", got, len(out))
	}
}

func TestAlbumBubbleLinesEqualWidthNarrowPane(t *testing.T) {
	// Narrow pane: the folded badge ("[n] 🎥 video 0:12") is wider than the tiny
	// preview, so actualW must account for the badge (plus its trailing gap) or the
	// badge row overflows and tears the bubble border.
	ml := NewMessageList(30, 22)
	ml.SetImage(11, image.NewRGBA(image.Rect(0, 0, 320, 240))) // video thumb cached
	ml.SetImage(22, image.NewRGBA(image.Rect(0, 0, 600, 800))) // photo cached
	parts := []domain.Message{
		{ID: 1, GroupedID: 100, SenderID: 7,
			Media:    &domain.MediaRef{Kind: domain.MediaVideo, Duration: 12},
			Document: &domain.DocumentRef{ID: 11, ThumbSize: "m"}},
		{ID: 2, GroupedID: 100, SenderID: 7,
			Media: &domain.MediaRef{Kind: domain.MediaPhoto}, Photo: &domain.PhotoRef{ID: 22}},
	}
	lines := ml.renderGroupBubble(parts, false)
	w0 := lipgloss.Width(lines[0])
	for i, ln := range lines {
		if lipgloss.Width(ln) != w0 {
			t.Fatalf("line %d width %d != line0 width %d (torn border)\n%q", i, lipgloss.Width(ln), w0, ln)
		}
	}
}
