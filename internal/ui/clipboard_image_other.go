//go:build !darwin

package ui

// noopClipboardImageReader always reports "no image", so platforms without a
// reader degrade silently to the text paste path.
type noopClipboardImageReader struct{}

func (noopClipboardImageReader) ReadImage() ([]byte, string, error) { return nil, "", nil }

// newOSClipboardImageReader returns the noop reader on platforms without a
// clipboard image reader yet (Linux/Windows), so paste degrades to text.
func newOSClipboardImageReader() clipboardImageReader { return noopClipboardImageReader{} }
