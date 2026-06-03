package logx

import (
	"context"
	"log/slog"
	"strings"

	"github.com/coldsmirk/vef-framework-go/logx"
)

type slogHandler struct {
	logger      logx.Logger
	attrs       []slog.Attr
	levelFilter logx.Level
}

func (s *slogHandler) Enabled(_ context.Context, level slog.Level) bool {
	logLevel := slogLevelToLogLevel(level)

	return s.logger.Enabled(logLevel) && logLevel >= s.levelFilter
}

func slogLevelToLogLevel(level slog.Level) logx.Level {
	switch {
	case level >= slog.LevelError:
		return logx.LevelError
	case level >= slog.LevelWarn:
		return logx.LevelWarn
	case level >= slog.LevelInfo:
		return logx.LevelInfo
	default:
		return logx.LevelDebug
	}
}

func (s *slogHandler) Handle(_ context.Context, record slog.Record) error {
	fields := make([]string, 0, record.NumAttrs()+len(s.attrs))
	for _, attr := range s.attrs {
		fields = appendAttr(fields, attr)
	}

	record.Attrs(func(attr slog.Attr) bool {
		fields = appendAttr(fields, attr)

		return true
	})

	fieldsValue := strings.Join(fields, " | ")
	if len(fields) > 0 {
		fieldsValue = " | " + fieldsValue
	}

	message := record.Message + fieldsValue
	switch slogLevelToLogLevel(record.Level) {
	case logx.LevelDebug:
		s.logger.Debug(message)
	case logx.LevelInfo:
		s.logger.Info(message)
	case logx.LevelWarn:
		s.logger.Warn(message)
	default:
		s.logger.Error(message)
	}

	return nil
}

func (s *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	merged := make([]slog.Attr, 0, len(s.attrs)+len(attrs))
	merged = append(merged, s.attrs...)
	merged = append(merged, attrs...)

	return &slogHandler{
		logger:      s.logger,
		attrs:       merged,
		levelFilter: s.levelFilter,
	}
}

func (s *slogHandler) WithGroup(name string) slog.Handler {
	return &slogHandler{
		logger:      s.logger.Named(name),
		attrs:       s.attrs,
		levelFilter: s.levelFilter,
	}
}

func appendAttr(fields []string, attr slog.Attr) []string {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return fields
	}

	if attr.Value.Kind() == slog.KindGroup {
		for _, sub := range attr.Value.Group() {
			if attr.Key != "" {
				sub = slog.Attr{Key: attr.Key + "." + sub.Key, Value: sub.Value}
			}

			fields = appendAttr(fields, sub)
		}

		return fields
	}

	// slog.Value.String renders every non-group kind canonically (scalars via
	// strconv, Duration/Time via their String, KindAny via fmt), so it covers
	// arbitrary values that a typed converter would otherwise drop.
	return append(fields, attr.Key+": "+attr.Value.String())
}

func NewSlogHandler(name string, callerSkip int, levelFilter ...logx.Level) slog.Handler {
	level := logx.LevelInfo
	if len(levelFilter) > 0 {
		level = levelFilter[0]
	}

	return &slogHandler{
		logger:      Named(name).WithCallerSkip(callerSkip),
		levelFilter: level,
	}
}

func NewSLogger(name string, callerSkip int, levelFilter ...logx.Level) *slog.Logger {
	return slog.New(NewSlogHandler(name, callerSkip, levelFilter...))
}
