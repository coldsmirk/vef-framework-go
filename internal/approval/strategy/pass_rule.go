package strategy

import "github.com/coldsmirk/vef-framework-go/approval"

// NewAllPassStrategy creates a new AllPassStrategy.
func NewAllPassStrategy() approval.PassRuleStrategy {
	return new(AllPassStrategy)
}

// AllPassStrategy requires all assignees to approve; any single rejection
// fails the node, so "everyone must agree" and "one veto rejects" are the
// same rule under this strategy.
type AllPassStrategy struct{}

func (*AllPassStrategy) Rule() approval.PassRule { return approval.PassAll }

func (*AllPassStrategy) Evaluate(ctx approval.PassRuleContext) approval.PassRuleResult {
	if ctx.RejectedCount > 0 {
		return approval.PassRuleRejected
	}

	if ctx.ApprovedCount == ctx.TotalCount && ctx.TotalCount > 0 {
		return approval.PassRulePassed
	}

	return approval.PassRulePending
}

// NewAnyPassStrategy creates a new AnyPassStrategy.
func NewAnyPassStrategy() approval.PassRuleStrategy {
	return new(AnyPassStrategy)
}

// AnyPassStrategy passes when at least one assignee approves.
type AnyPassStrategy struct{}

func (*AnyPassStrategy) Rule() approval.PassRule { return approval.PassAny }

func (*AnyPassStrategy) Evaluate(ctx approval.PassRuleContext) approval.PassRuleResult {
	if ctx.ApprovedCount > 0 {
		return approval.PassRulePassed
	}

	if ctx.RejectedCount == ctx.TotalCount && ctx.TotalCount > 0 {
		return approval.PassRuleRejected
	}

	return approval.PassRulePending
}

// NewRatioPassStrategy creates a new RatioPassStrategy.
func NewRatioPassStrategy() approval.PassRuleStrategy {
	return new(RatioPassStrategy)
}

// RatioPassStrategy passes when approval ratio meets threshold.
type RatioPassStrategy struct{}

func (*RatioPassStrategy) Rule() approval.PassRule { return approval.PassRatio }

func (*RatioPassStrategy) Evaluate(ctx approval.PassRuleContext) approval.PassRuleResult {
	if ctx.TotalCount == 0 {
		return approval.PassRulePending
	}

	ratio := float64(ctx.ApprovedCount) / float64(ctx.TotalCount) * 100.0
	if ratio >= ctx.PassRatio {
		return approval.PassRulePassed
	}

	maxRatio := float64(ctx.TotalCount-ctx.RejectedCount) / float64(ctx.TotalCount) * 100.0
	if maxRatio < ctx.PassRatio {
		return approval.PassRuleRejected
	}

	return approval.PassRulePending
}
