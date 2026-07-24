package components

import (
	"strconv"

	"github.com/sorokin-vladimir/tele/internal/markup"
	"github.com/sorokin-vladimir/tele/internal/store"
)

// OpenTargetKind classifies something in a message that the "open" action can act
// on: a photo or video (in-app viewer) or a link (default browser).
type OpenTargetKind int

const (
	OpenTargetPhoto OpenTargetKind = iota
	OpenTargetVideo
	OpenTargetLink
)

// OpenTarget is one openable item of a message. For links, URL is the normalized
// target handed to the browser; for media it is empty (the root resolves the ref
// from the current selection).
type OpenTarget struct {
	Kind  OpenTargetKind
	Label string
	URL   string
	// PartIndex is the 1-based album part this target opens, matching the bubble
	// index badges and the modal paging order. 0 for a single-message media target
	// or a link.
	PartIndex int
}

// MessageOpenTargets returns the openable targets of a message in display order:
// media (photo/video) first, then links in text order. Voice, audio, GIF, files,
// stickers, phone numbers and bank cards are not openable and are omitted.
func MessageOpenTargets(msg store.Message) []OpenTarget {
	var targets []OpenTarget

	switch {
	case msg.Photo != nil:
		targets = append(targets, OpenTarget{Kind: OpenTargetPhoto, Label: "Photo"})
	case msg.Media != nil && msg.Media.Kind.IsVideo() && msg.Document != nil:
		targets = append(targets, OpenTarget{Kind: OpenTargetVideo, Label: "Video"})
	}

	targets = append(targets, linkTargets(msg.Text, msg.Entities)...)
	return targets
}

// GroupOpenTargets lists the openable targets of an album: one entry per media
// part labelled with its 1-based index (matching the bubble badges and the modal
// paging order), followed by the links found in the album caption.
func GroupOpenTargets(parts []store.Message) []OpenTarget {
	var targets []OpenTarget
	n := 0
	for _, p := range parts {
		var kind OpenTargetKind
		var stem string
		switch {
		case p.Photo != nil:
			kind, stem = OpenTargetPhoto, "Photo"
		case p.Media != nil && p.Media.Kind.IsVideo() && p.Document != nil:
			kind, stem = OpenTargetVideo, "Video"
		case p.Document != nil:
			// A generic file: routed to the download/open-external path by the root,
			// distinguished from a photo by its label and non-zero PartIndex.
			kind, stem = OpenTargetPhoto, "File"
		default:
			continue
		}
		n++
		targets = append(targets, OpenTarget{Kind: kind, Label: stem + " " + strconv.Itoa(n), PartIndex: n})
	}
	// Links come from the caption-bearing part.
	for _, p := range parts {
		if p.Text != "" {
			targets = append(targets, linkTargets(p.Text, p.Entities)...)
			break
		}
	}
	return targets
}

// linkTargets extracts openable link targets (url/email/text_url) from text in
// entity order. Entity offsets are UTF-16 code units.
func linkTargets(text string, entities []store.MessageEntity) []OpenTarget {
	if len(entities) == 0 {
		return nil
	}
	runes := []rune(text)
	n := len(runes)
	var out []OpenTarget
	for _, e := range entities {
		switch e.Type {
		case "url", "email", "text_url":
		default:
			continue
		}
		start := markup.UTF16ToRuneIndex(text, e.Offset)
		end := markup.UTF16ToRuneIndex(text, e.Offset+e.Length)
		if start >= n || start >= end {
			continue
		}
		if end > n {
			end = n
		}
		visible := string(runes[start:end])
		switch e.Type {
		case "text_url":
			out = append(out, OpenTarget{Kind: OpenTargetLink, Label: visible + " → " + e.URL, URL: e.URL})
		case "url", "email":
			out = append(out, OpenTarget{Kind: OpenTargetLink, Label: visible, URL: normalizeLinkTarget(e.Type, visible)})
		}
	}
	return out
}
