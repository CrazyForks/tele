package tg

import (
	"runtime/debug"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// gotdModule is the import path of the MTProto library, looked up in the build
// info to report which version this binary was linked against.
const gotdModule = "github.com/gotd/td"

// withoutField returns a logger that discards any field named key, whether it
// arrives through With or at the log site.
//
// telegram.NewClient stamps its own module version onto the logger it is handed
// ("v"). That key is where the application version belongs, and a duplicate in
// the JSON output would be resolved in gotd's favour by any reader that keeps
// the last occurrence. The library version is logged once at startup instead,
// see gotdVersion.
func withoutField(log *zap.Logger, key string) *zap.Logger {
	return log.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return dropFieldCore{Core: c, key: key}
	}))
}

type dropFieldCore struct {
	zapcore.Core
	key string
}

func (c dropFieldCore) With(fields []zapcore.Field) zapcore.Core {
	return dropFieldCore{Core: c.Core.With(c.filter(fields)), key: c.key}
}

// Check routes the entry through this core rather than the wrapped one, which
// is what keeps Write below in the path for log-site fields.
func (c dropFieldCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

func (c dropFieldCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	return c.Core.Write(ent, c.filter(fields))
}

func (c dropFieldCore) filter(fields []zapcore.Field) []zapcore.Field {
	kept := make([]zapcore.Field, 0, len(fields))
	for _, f := range fields {
		if f.Key == c.key {
			continue
		}
		kept = append(kept, f)
	}
	return kept
}

// gotdVersion reports the gotd version this binary was built against. It is
// logged once at startup because withoutField strips the per-line field gotd
// would otherwise attach.
func gotdVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return gotdVersionFrom(info)
}

func gotdVersionFrom(info *debug.BuildInfo) string {
	if info == nil {
		return ""
	}
	for _, dep := range info.Deps {
		if dep.Path == gotdModule {
			return dep.Version
		}
	}
	return ""
}
