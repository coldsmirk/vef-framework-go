package logx

import (
	"context"
	"log/slog"
	"strings"

	"github.com/spf13/cast"

	"github.com/coldsmirk/vef-framework-go/logx"
)

type slogHandler struct {
	logger      logx.Logger
	attrs       []slog.Attr
	levelFilter logx.Level
}

func (s slogHandler) Enabled(_ context.Context, level slog.Level) bool {
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

func (s slogHandler) Handle(_ context.Context, record slog.Record) error {
	fields := make([]string, 0, record.NumAttrs()+len(s.attrs))

	record.Attrs(func(attr slog.Attr) bool {
		if field := formatAttr(attr); field != "" {
			fields = append(fields, field)
		}

		return true
	})

	fieldsValue := strings.Join(fields, " | ")
	if len(fields) > 0 {
		fieldsValue = " | " + fieldsValue
	}

	message := record.Message + fieldsValue
	switch record.Level {
	case slog.LevelDebug:
		s.logger.Debug(message)
	case slog.LevelInfo:
		s.logger.Info(message)
	case slog.LevelWarn:
		s.logger.Warn(message)
	case slog.LevelError:
		s.logger.Error(message)
	}

	return nil
}

func (s slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &slogHandler{
		logger:      s.logger,
		attrs:       append(s.attrs, attrs...),
		levelFilter: s.levelFilter,
	}
}

func (s slogHandler) WithGroup(name string) slog.Handler {
	return &slogHandler{
		logger:      s.logger.Named(name),
		attrs:       s.attrs,
		levelFilter: s.levelFilter,
	}
}

func formatAttr(attr slog.Attr) string {
	var value string

	switch attr.Value.Kind() {
	case slog.KindString:
		value = attr.Value.String()
	case slog.KindInt64:
		value = cast.ToString(attr.Value.Int64())
	case slog.KindUint64:
		value = cast.ToString(attr.Value.Uint64())
	case slog.KindFloat64:
		value = cast.ToString(attr.Value.Float64())
	case slog.KindBool:
		value = cast.ToString(attr.Value.Bool())
	case slog.KindDuration:
		value = cast.ToString(attr.Value.Duration())
	case slog.KindTime:
		value = cast.ToString(attr.Value.Time())
	case slog.KindAny:
		value = cast.ToString(attr.Value.Any())
	default:
		return ""
	}

	return attr.Key + ": " + value
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
