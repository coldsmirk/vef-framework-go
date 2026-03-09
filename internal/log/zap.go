package log //nolint:revive // intentional package name

import (
	"fmt"
	"time"

	"github.com/muesli/termenv"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func newZapLogger(level zapcore.Level) *zap.SugaredLogger {
	output := termenv.DefaultOutput()
	config := zap.Config{
		Level:       zap.NewAtomicLevelAt(level),
		Development: false,
		Encoding:    "console",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:       "time",
			LevelKey:      "level",
			NameKey:       "logger",
			FunctionKey:   zapcore.OmitKey,
			MessageKey:    "message",
			StacktraceKey: zapcore.OmitKey,
			CallerKey:     zapcore.OmitKey,
			LineEnding:    zapcore.DefaultLineEnding,
			EncodeLevel:   zapcore.CapitalColorLevelEncoder,
			EncodeTime: func() zapcore.TimeEncoder {
				layout := time.DateOnly + "T" + time.TimeOnly + ".000"

				return func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
					enc.AppendString(
						output.String(t.Format(layout)).
							Foreground(termenv.ANSIBrightBlack).
							String(),
					)
				}
			}(),
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeName: func(name string, enc zapcore.PrimitiveArrayEncoder) {
				enc.AppendString(
					output.String("[" + name + "]").
						Foreground(termenv.ANSIBrightMagenta).
						String(),
				)
			},
		},
		DisableStacktrace: true,
		OutputPaths:       []string{"stdout"},
		ErrorOutputPaths:  []string{"stderr"},
	}

	logger, err := config.Build(zap.WithCaller(false))
	if err != nil {
		panic(
			fmt.Errorf("failed to build zap logger: %w", err),
		)
	}

	return logger.Sugar()
}
