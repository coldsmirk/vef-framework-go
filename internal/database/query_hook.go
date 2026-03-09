package database

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

	"github.com/coldsmirk/vef-framework-go/internal/database/sqlguard"
	"github.com/coldsmirk/vef-framework-go/log"
)

// whitespaceRegex matches consecutive whitespace characters (spaces, tabs, newlines).
var whitespaceRegex = regexp.MustCompile(`\s+`)

// guardErrorStashKey is the stash key for storing guard errors.
const guardErrorStashKey = "__sqlguard_error"

type queryHook struct {
	logger   log.Logger
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

	elapsedStyle := qh.formatElapsedTime(elapsed)
	operationStyle := qh.formatOperation(event.Operation())
	queryStyle := qh.formatQuery(event.Query)

	displayErr := guardErr
	if displayErr == nil {
		displayErr = event.Err
	}

	if displayErr != nil && !errors.Is(displayErr, sql.ErrNoRows) {
		errorStyle := qh.output.String(displayErr.Error()).Foreground(termenv.ANSIRed)
		qh.logger.Error(operationStyle.String() + elapsedStyle.String() + " " + queryStyle.String() + " " + errorStyle.String())

		return
	}

	message := operationStyle.String() + elapsedStyle.String() + " " + queryStyle.String()
	if elapsed >= 500 {
		qh.logger.Warn(message)
	} else {
		qh.logger.Info(message)
	}
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

func addQueryHook(db *bun.DB, logger log.Logger, guardConfig *sqlguard.Config) {
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
