package sequence

import (
	"context"
	"strconv"
	"strings"

	"github.com/coldsmirk/vef-framework-go/sequence"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// Engine implements sequence.Generator using a pluggable Store backend.
type Engine struct {
	store sequence.Store
}

// NewGenerator creates a new sequence generator.
func NewGenerator(store sequence.Store) sequence.Generator {
	return &Engine{store: store}
}

func (e *Engine) Generate(ctx context.Context, key string) (string, error) {
	results, err := e.GenerateN(ctx, key, 1)
	if err != nil {
		return "", err
	}

	return results[0], nil
}

func (e *Engine) GenerateN(ctx context.Context, key string, count int) ([]string, error) {
	if count < 1 {
		return nil, sequence.ErrInvalidCount
	}

	now := timex.Now()

	rule, newValue, err := e.store.Reserve(ctx, key, count, now)
	if err != nil {
		return nil, err
	}

	return buildSerialNumbers(rule, newValue, count, now), nil
}

// buildSerialNumbers constructs serial number strings for a batch.
// NewValue is the final counter value after incrementing; we work backwards to get each value.
func buildSerialNumbers(rule *sequence.Rule, newValue, count int, now timex.DateTime) []string {
	results := make([]string, count)
	datePart := sequence.FormatDate(now, rule.DateFormat)

	for i := range count {
		// Values in the batch: newValue - (count-1-i)*step
		seqValue := newValue - (count-1-i)*rule.SeqStep
		results[i] = buildSingleSerialNo(rule, seqValue, datePart)
	}

	return results
}

// buildSingleSerialNo constructs a single serial number string.
func buildSingleSerialNo(rule *sequence.Rule, seqValue int, datePart string) string {
	var sb strings.Builder
	sb.Grow(len(rule.Prefix) + len(datePart) + rule.SeqLength + len(rule.Suffix))

	sb.WriteString(rule.Prefix)
	sb.WriteString(datePart)

	// Zero-padded sequence number without fmt overhead
	numStr := strconv.Itoa(seqValue)
	for range rule.SeqLength - len(numStr) {
		sb.WriteByte('0')
	}

	sb.WriteString(numStr)

	sb.WriteString(rule.Suffix)

	return sb.String()
}
