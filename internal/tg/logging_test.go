package tg

import (
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestWithoutField_DropsTheKeyGotdStampsOnItsLogger(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	root := zap.New(core).With(zap.String("v", "1.9.1"))

	// telegram.NewClient stamps the gotd module version onto the logger it is
	// given, which would otherwise shadow the application version.
	gotd := withoutField(root, "v").With(zap.String("v", "v0.160.0"))
	gotd.Info("Closed")

	entries := logs.All()
	require.Len(t, entries, 1)
	assert.Equal(t, map[string]any{"v": "1.9.1"}, entries[0].ContextMap())
}

func TestWithoutField_KeepsEveryOtherField(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)

	gotd := withoutField(zap.New(core), "v").With(zap.String("v", "v0.160.0"), zap.Int("conn_id", 0))
	gotd.Info("Close called", zap.Int("dc_id", 2))

	entries := logs.All()
	require.Len(t, entries, 1)
	assert.Equal(t, map[string]any{"conn_id": int64(0), "dc_id": int64(2)}, entries[0].ContextMap())
}

func TestWithoutField_DropsTheKeyAtTheLogSiteToo(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)

	withoutField(zap.New(core), "v").Info("Closed", zap.String("v", "v0.160.0"))

	entries := logs.All()
	require.Len(t, entries, 1)
	assert.Empty(t, entries[0].ContextMap())
}

func TestWithoutField_KeepsDroppingAcrossNamedSubLoggers(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)

	gotd := withoutField(zap.New(core), "v").With(zap.String("v", "v0.160.0"))
	gotd.Named("conn").Named("mtproto").Info("Connected")

	entries := logs.All()
	require.Len(t, entries, 1)
	assert.Equal(t, "conn.mtproto", entries[0].LoggerName)
	assert.Empty(t, entries[0].ContextMap())
}

// The build info of a test binary carries no dependencies at all, so the lookup
// is tested against a synthetic one; gotdVersion itself only reads the real one.
func TestGotdVersionFrom_ReportsTheModuleTheBinaryWasBuiltAgainst(t *testing.T) {
	info := &debug.BuildInfo{Deps: []*debug.Module{
		{Path: "go.uber.org/zap", Version: "v1.27.0"},
		{Path: gotdModule, Version: "v0.160.0"},
	}}

	assert.Equal(t, "v0.160.0", gotdVersionFrom(info))
}

func TestGotdVersionFrom_IsEmptyWhenBuildInfoIsUnavailable(t *testing.T) {
	assert.Empty(t, gotdVersionFrom(nil))
}
