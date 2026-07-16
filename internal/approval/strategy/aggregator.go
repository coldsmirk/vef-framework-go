package strategy

import (
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// expectedAggregates lists the built-in AggregateKind values the framework
// guarantees have a registered aggregator. Missing registrations surface at
// boot via ValidateBuiltinAggregators.
var expectedAggregates = []approval.AggregateKind{
	approval.AggregateSum,
	approval.AggregateCount,
	approval.AggregateAvg,
}

// ValidateBuiltinAggregators asserts every built-in aggregate kind has a
// registered implementation. The production strategy.Module invokes it during
// boot, mirroring StrategyRegistry.ValidateBuiltins.
func ValidateBuiltinAggregators(aggregators []approval.Aggregator) error {
	registered := make(map[approval.AggregateKind]struct{}, len(aggregators))
	for _, agg := range aggregators {
		registered[agg.Kind()] = struct{}{}
	}

	for _, kind := range expectedAggregates {
		if _, ok := registered[kind]; !ok {
			return fmt.Errorf("%w: %s", errBuiltinAggregatorMissing, kind)
		}
	}

	return nil
}

// SumAggregator folds a numeric column into its total. An empty table sums
// to 0 — the SQL convention for SUM over no rows.
type SumAggregator struct{}

// NewSumAggregator creates the sum aggregator.
func NewSumAggregator() approval.Aggregator { return new(SumAggregator) }

func (*SumAggregator) Kind() approval.AggregateKind { return approval.AggregateSum }

func (*SumAggregator) Fold(values []float64, _ int) (float64, bool) {
	var sum float64
	for _, v := range values {
		sum += v
	}

	return sum, true
}

// CountAggregator folds a table into its row count. It works on rows, not a
// column, so conditions using it leave Column empty.
type CountAggregator struct{}

// NewCountAggregator creates the count aggregator.
func NewCountAggregator() approval.Aggregator { return new(CountAggregator) }

func (*CountAggregator) Kind() approval.AggregateKind { return approval.AggregateCount }

func (*CountAggregator) Fold(_ []float64, rowCount int) (float64, bool) {
	return float64(rowCount), true
}

// AvgAggregator folds a numeric column into its mean. The average of zero
// values is undefined — like SQL's AVG returning NULL, the condition then
// matches nothing instead of comparing against a fabricated 0.
type AvgAggregator struct{}

// NewAvgAggregator creates the avg aggregator.
func NewAvgAggregator() approval.Aggregator { return new(AvgAggregator) }

func (*AvgAggregator) Kind() approval.AggregateKind { return approval.AggregateAvg }

func (*AvgAggregator) Fold(values []float64, _ int) (float64, bool) {
	if len(values) == 0 {
		return 0, false
	}

	var sum float64
	for _, v := range values {
		sum += v
	}

	return sum / float64(len(values)), true
}
