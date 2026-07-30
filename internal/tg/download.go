package tg

// Downloads are streams, not images: the owner writes them to disk and clients
// decode from the file, so nothing here decodes and no image decoder is
// registered in this package any more (#196).
import (
	"context"
	"fmt"
	"io"

	"github.com/gotd/td/telegram/downloader"
	gotdtg "github.com/gotd/td/tg"

	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/telerr"
)

// DownloadPhotoToFile streams the raw photo bytes (the size named by
// ref.ThumbSize) directly into dst without decoding, so a photo can be saved to
// disk at full quality. Mirrors DownloadDocumentToFile.
func (c *GotdClient) DownloadPhotoToFile(ctx context.Context, ref domain.PhotoRef, dst io.Writer) error {
	api, err := c.acquireAPI()
	if err != nil {
		return err
	}

	loc := &gotdtg.InputPhotoFileLocation{
		ID:            ref.ID,
		AccessHash:    ref.AccessHash,
		FileReference: ref.FileReference,
		ThumbSize:     ref.ThumbSize,
	}

	d := downloader.NewDownloader()
	if _, err := d.Download(api, loc).Stream(ctx, dst); err != nil {
		return fmt.Errorf("download photo %d: %w", ref.ID, err)
	}
	return nil
}

// DownloadDocumentToFile streams the full document into dst without buffering
// the whole file in memory, so it stays bounded regardless of file size.
func (c *GotdClient) DownloadDocumentToFile(ctx context.Context, ref domain.DocumentRef, dst io.Writer) error {
	api, err := c.acquireAPI()
	if err != nil {
		return err
	}

	loc := &gotdtg.InputDocumentFileLocation{
		ID:            ref.ID,
		AccessHash:    ref.AccessHash,
		FileReference: ref.FileReference,
	}

	d := downloader.NewDownloader()
	if _, err := d.Download(api, loc).Stream(ctx, dst); err != nil {
		return fmt.Errorf("download document %d: %w", ref.ID, err)
	}
	return nil
}

// DownloadDocumentThumbToFile streams the document's thumbnail (the size named
// by ref.ThumbSize) into dst. It is separate from DownloadDocumentToFile
// because that one deliberately ignores ThumbSize and always streams the full
// file; a poster frame needs the thumbnail location instead.
func (c *GotdClient) DownloadDocumentThumbToFile(ctx context.Context, ref domain.DocumentRef, dst io.Writer) error {
	if ref.ThumbSize == "" {
		return &telerr.Error{Kind: telerr.NotFound, Op: "download document thumb", Detail: "no thumbnail"}
	}
	api, err := c.acquireAPI()
	if err != nil {
		return err
	}

	loc := &gotdtg.InputDocumentFileLocation{
		ID:            ref.ID,
		AccessHash:    ref.AccessHash,
		FileReference: ref.FileReference,
		ThumbSize:     ref.ThumbSize,
	}

	d := downloader.NewDownloader()
	if _, err := d.Download(api, loc).Stream(ctx, dst); err != nil {
		return fmt.Errorf("download document thumb %d: %w", ref.ID, err)
	}
	return nil
}
