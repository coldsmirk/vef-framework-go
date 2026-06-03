package logx

import (
	"fmt"
	"time"

	"github.com/muesli/termenv"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const timeLayout = time.DateOnly + "T" + time.TimeOnly + ".000"

func newZapLogger(level zap.AtomicLevel) *zap.SugaredLogger {
	output := termenv.DefaultOutput()
	config := zap.Config{
		Level:       level,
		Development: false,
		Encoding:    "console",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "time",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "message",
			StacktraceKey:  zapcore.OmitKey,
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalColorLevelEncoder,
			EncodeTime:     encodeTime(output),
			EncodeCaller:   encodeCaller(output),
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeName:     encodeName(output),
		},
		DisableStacktrace: true,
		OutputPaths:       []string{"stdout"},
		ErrorOutputPaths:  []string{"stderr"},
	}

	logger, err := config.Build()
	if err != nil {
		panic(
			fmt.Errorf("failed to build zap logger: %w", err),
		)
	}

	return logger.Sugar()
}

func encodeTime(output *termenv.Output) zapcore.TimeEncoder {
	return func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(dim(output, t.Format(timeLayout)))
	}
}

func encodeCaller(output *termenv.Output) zapcore.CallerEncoder {
	return func(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(dim(output, caller.TrimmedPath()))
	}
}

func encodeName(output *termenv.Output) zapcore.NameEncoder {
	return func(name string, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(
			output.String("[" + name + "]").
				Foreground(termenv.ANSIBrightMagenta).
				String(),
		)
	}
}

func dim(output *termenv.Output, text string) string {
	return output.String(text).Foreground(termenv.ANSIBrightBlack).String()
}
