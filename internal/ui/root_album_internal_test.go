package ui

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/store"
)

func att(name string, kind store.MediaKind) pendingAttachment {
	return pendingAttachment{name: name, kind: kind, sendAs: kind}
}

func attNames(group []pendingAttachment) []string {
	out := make([]string, 0, len(group))
	for _, a := range group {
		out = append(out, a.name)
	}
	return out
}

func TestPartitionAlbums(t *testing.T) {
	tests := []struct {
		name string
		in   []pendingAttachment
		want [][]string
	}{
		{
			name: "empty",
			in:   nil,
			want: nil,
		},
		{
			name: "single file is one group",
			in:   []pendingAttachment{att("a.jpg", store.MediaPhoto)},
			want: [][]string{{"a.jpg"}},
		},
		{
			name: "photos and videos share one album",
			in: []pendingAttachment{
				att("a.jpg", store.MediaPhoto), att("b.mp4", store.MediaVideo),
			},
			want: [][]string{{"a.jpg", "b.mp4"}},
		},
		{
			name: "documents split off, visual group first by first occurrence",
			in: []pendingAttachment{
				att("a.jpg", store.MediaPhoto), att("c.pdf", store.MediaFile), att("b.mp4", store.MediaVideo),
			},
			want: [][]string{{"a.jpg", "b.mp4"}, {"c.pdf"}},
		},
		{
			name: "group order follows the first occurrence of each class",
			in: []pendingAttachment{
				att("c.pdf", store.MediaFile), att("a.jpg", store.MediaPhoto),
			},
			want: [][]string{{"c.pdf"}, {"a.jpg"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := partitionAlbums(tt.in)
			require.Len(t, got, len(tt.want))
			for i := range tt.want {
				assert.Equal(t, tt.want[i], attNames(got[i]))
			}
		})
	}
}

func TestPartitionAlbums_ChunksAtTen(t *testing.T) {
	in := make([]pendingAttachment, 0, 12)
	for i := 0; i < 12; i++ {
		in = append(in, att(fmt.Sprintf("p%02d.jpg", i), store.MediaPhoto))
	}

	got := partitionAlbums(in)

	require.Len(t, got, 2)
	assert.Len(t, got[0], 10)
	assert.Len(t, got[1], 2)
	assert.Equal(t, "p00.jpg", got[0][0].name)
	assert.Equal(t, "p10.jpg", got[1][0].name)
}

func TestPartitionAlbums_SendAsFileMovesPhotoToDocumentClass(t *testing.T) {
	photoAsFile := att("a.jpg", store.MediaPhoto)
	photoAsFile.sendAs = store.MediaFile

	got := partitionAlbums([]pendingAttachment{photoAsFile, att("b.pdf", store.MediaFile)})

	require.Len(t, got, 1, "a photo sent as a file belongs with the documents")
	assert.Equal(t, []string{"a.jpg", "b.pdf"}, attNames(got[0]))
}
