package cron

// cronLogger implements gocron.Logger interface to integrate with the framework's logging system.
type cronLogger struct{}

func (*cronLogger) Debug(msg string, args ...any) {
	logger.Debugf(msg, args...)
}

func (*cronLogger) Error(msg string, args ...any) {
	logger.Errorf(msg, args...)
}

func (*cronLogger) Info(msg string, args ...any) {
	logger.Infof(msg, args...)
}

func (*cronLogger) Warn(msg string, args ...any) {
	logger.Warnf(msg, args...)
}

func newCronLogger() *cronLogger {
	return &cronLogger{}
}
