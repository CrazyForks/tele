package app

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	"go.uber.org/zap"

	"github.com/sorokin-vladimir/tele/internal/config"
	"github.com/sorokin-vladimir/tele/internal/mediacache"
)

// tmpCacheBytes bounds the media cache when the user asked for no persistent
// one. The files still have to land somewhere for a fetch to return a path, so
// they land in the run's temp directory and go away with it.
const tmpCacheBytes = 64 << 20

// accountSegment is the per-account directory name inside the shared cache
// directory: the first 12 hex digits of the SHA-256 of the state directory.
// Stable across runs, filename-safe, and it names nothing about the account.
func accountSegment(stateDir string) string {
	sum := sha256.Sum256([]byte(stateDir))
	return hex.EncodeToString(sum[:])[:12]
}

// mediaCacheDir is where this account's media cache lives.
func mediaCacheDir(stateDir string) (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "tele", accountSegment(stateDir), "media"), nil
}

// removeLegacyMediaCache deletes the pre-#196 cache directory, which was shared
// by every account and is now unreachable. It is a sibling of the per-account
// directories, never a parent of one, so removing it cannot touch a live cache.
func removeLegacyMediaCache(log *zap.Logger) {
	base, err := os.UserCacheDir()
	if err != nil {
		return
	}
	legacy := filepath.Join(base, "tele", "media")
	if err := os.RemoveAll(legacy); err != nil {
		log.Debug("could not remove the legacy media cache", zap.String("dir", legacy), zap.Error(err))
	}
}

// openMediaCache builds the account's media cache. photos.disk_cache_size == 0
// means "keep nothing between runs": the cache goes into the run's temp
// directory under a fixed bound and is deleted with it on exit.
func openMediaCache(cfg *config.Config, tmpDir string, log *zap.Logger) (*mediacache.Cache, error) {
	if cfg.Photos.DiskCacheSize <= 0 {
		return mediacache.New(filepath.Join(tmpDir, "media"), tmpCacheBytes)
	}
	dir, err := mediaCacheDir(cfg.StateDir)
	if err != nil {
		log.Warn("no user cache directory; caching media in the temp directory instead", zap.Error(err))
		return mediacache.New(filepath.Join(tmpDir, "media"), tmpCacheBytes)
	}
	return mediacache.New(dir, cfg.Photos.DiskCacheSize)
}
