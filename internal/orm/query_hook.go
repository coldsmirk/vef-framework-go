package orm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/muesli/termenv"
	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/internal/orm/sqlguard"
	"github.com/coldsmirk/vef-framework-go/logx"
)

// whitespaceRegex matches consecutive whitespace characters (spaces, tabs, newlines).
var whitespaceRegex = regexp.MustCompile(`\s+`)

// guardErrorStashKey is the stash key for storing guard errors.
const guardErrorStashKey = "__sqlguard_error"

// slowQueryThresholdMillis is the elapsed time at or above which a query is
// logged at Warn instead of Info.
const slowQueryThresholdMillis = 500

type queryHook struct {
	logger   logx.Logger
	output   *termenv.Output
	sqlGuard *sqlguard.Guard
}

func (qh *queryHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	if qh.sqlGuard == nil || sqlguard.IsWhitelisted(ctx) {
		return ctx
	}

	if err := qh.sqlGuard.Check(event.Query); err != nil {
		if event.Stash == nil {
			event.Stash = make(map[any]any)
		}

		event.Stash[guardErrorStashKey] = err

		cancelCtx, cancel := context.WithCancelCause(ctx)
		cancel(err)

		return cancelCtx
	}

	return ctx
}

func (qh *queryHook) AfterQuery(_ context.Context, event *bun.QueryEvent) {
	guardErr := qh.extractGuardError(event)
	elapsed := time.Since(event.StartTime).Milliseconds()

	displayErr := guardErr
	if displayErr == nil {
		displayErr = event.Err
	}

	if displayErr != nil && !errors.Is(displayErr, sql.ErrNoRows) {
		errorStyle := qh.output.String(displayErr.Error()).Foreground(termenv.ANSIRed)
		qh.logger.Error(qh.formatPrefix(elapsed, event.Operation(), event.Query) + " " + errorStyle.String())

		return
	}

	// Skip the regex normalization and ANSI styling entirely when the target
	// level is disabled — AfterQuery is the per-query hot path and the formatted
	// message would otherwise be built only to be discarded.
	level := logx.LevelInfo
	if elapsed >= slowQueryThresholdMillis {
		level = logx.LevelWarn
	}

	if !qh.logger.Enabled(level) {
		return
	}

	message := qh.formatPrefix(elapsed, event.Operation(), event.Query)
	if level == logx.LevelWarn {
		qh.logger.Warn(message)
	} else {
		qh.logger.Info(message)
	}
}

// formatPrefix renders the styled "operation elapsed query" prefix shared by the
// info, warn, and error log paths.
func (qh *queryHook) formatPrefix(elapsed int64, operation, query string) string {
	return qh.formatOperation(operation).String() +
		qh.formatElapsedTime(elapsed).String() + " " +
		qh.formatQuery(query).String()
}

func (*queryHook) extractGuardError(event *bun.QueryEvent) error {
	if event.Stash == nil {
		return nil
	}

	err, _ := event.Stash[guardErrorStashKey].(error)

	return err
}

func (qh *queryHook) formatElapsedTime(elapsed int64) termenv.Style {
	style := qh.output.String(fmt.Sprintf("%6d ms", elapsed))

	switch {
	case elapsed >= 1000:
		return style.Bold().Foreground(termenv.ANSIRed)
	case elapsed >= 500:
		return style.Bold().Foreground(termenv.ANSIYellow)
	case elapsed >= 200:
		return style.Foreground(termenv.ANSIBlue)
	default:
		return style.Foreground(termenv.ANSIGreen)
	}
}

func (qh *queryHook) formatOperation(operation string) termenv.Style {
	style := qh.output.String(fmt.Sprintf(" %-8s", operation)).Bold()

	switch operation {
	case "SELECT":
		return style.Foreground(termenv.ANSIGreen)
	case "INSERT":
		return style.Foreground(termenv.ANSIBlue)
	case "UPDATE":
		return style.Foreground(termenv.ANSIYellow)
	case "DELETE":
		return style.Foreground(termenv.ANSIMagenta)
	default:
		return style.Foreground(termenv.ANSICyan)
	}
}

func (qh *queryHook) formatQuery(query string) termenv.Style {
	normalized := strings.TrimSpace(whitespaceRegex.ReplaceAllString(query, " "))

	return qh.output.String(normalized).Foreground(termenv.ANSIBrightBlack)
}

func addQueryHook(db *bun.DB, logger logx.Logger, guardConfig *sqlguard.Config) {
	var guard *sqlguard.Guard
	if guardConfig != nil && guardConfig.Enabled {
		guard = sqlguard.NewGuard(logger)
	}

	db.AddQueryHook(&queryHook{
		logger:   logger,
		output:   termenv.DefaultOutput(),
		sqlGuard: guard,
	})
}
