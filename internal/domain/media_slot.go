package domain

// MediaSlot names a piece of media hanging off a message. It is how a client
// asks for a file: the client names what it sees on screen, and the owner
// resolves that to a Telegram file location. A client never handles a file
// reference itself (#196).
type MediaSlot int

const (
	// PhotoThumb is the inline preview size stored on the photo reference.
	PhotoThumb MediaSlot = iota
	// PhotoFull is the largest available size, for the viewer and for saving.
	PhotoFull
	// DocThumb is a document's poster frame: the video or GIF preview.
	DocThumb
	// DocFull is the document itself: a sticker, a voice note, a video, a file.
	DocFull
)

func (s MediaSlot) String() string {
	switch s {
	case PhotoThumb:
		return "photo_thumb"
	case PhotoFull:
		return "photo_full"
	case DocThumb:
		return "doc_thumb"
	case DocFull:
		return "doc_full"
	}
	return "unknown"
}
