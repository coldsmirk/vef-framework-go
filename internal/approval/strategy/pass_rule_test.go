package strategy

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// TestAllPassStrategy tests all pass strategy scenarios.
func TestAllPassStrategy(t *testing.T) {
	s := NewAllPassStrategy()
	assert.Equal(t, approval.PassAll, s.Rule(), "Rule should be PassAll")

	tests := []struct {
		name     string
		ctx      approval.PassRuleContext
		expected approval.PassRuleResult
	}{
		{"AllApproved", approval.PassRuleContext{ApprovedCount: 3, RejectedCount: 0, TotalCount: 3}, approval.PassRulePassed},
		{"HasRejection", approval.PassRuleContext{ApprovedCount: 2, RejectedCount: 1, TotalCount: 3}, approval.PassRuleRejected},
		{"PartialApproved", approval.PassRuleContext{ApprovedCount: 1, RejectedCount: 0, TotalCount: 3}, approval.PassRulePending},
		{"EmptyTasks", approval.PassRuleContext{ApprovedCount: 0, RejectedCount: 0, TotalCount: 0}, approval.PassRulePending},
		{"SingleApproved", approval.PassRuleContext{ApprovedCount: 1, RejectedCount: 0, TotalCount: 1}, approval.PassRulePassed},
		{"SingleRejected", approval.PassRuleContext{ApprovedCount: 0, RejectedCount: 1, TotalCount: 1}, approval.PassRuleRejected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, s.Evaluate(tt.ctx), "Evaluate result should match expected")
		})
	}
}

// TestOnePassStrategy tests one pass strategy scenarios.
func TestOnePassStrategy(t *testing.T) {
	s := NewOnePassStrategy()
	assert.Equal(t, approval.PassAny, s.Rule(), "Rule should be PassAny")

	tests := []struct {
		name     string
		ctx      approval.PassRuleContext
		expected approval.PassRuleResult
	}{
		{"OneApproved", approval.PassRuleContext{ApprovedCount: 1, RejectedCount: 0, TotalCount: 3}, approval.PassRulePassed},
		{"ApprovedWithRejections", approval.PassRuleContext{ApprovedCount: 1, RejectedCount: 2, TotalCount: 3}, approval.PassRulePassed},
		{"AllRejected", approval.PassRuleContext{ApprovedCount: 0, RejectedCount: 3, TotalCount: 3}, approval.PassRuleRejected},
		{"NoCompleted", approval.PassRuleContext{ApprovedCount: 0, RejectedCount: 0, TotalCount: 3}, approval.PassRulePending},
		{"PartialRejected", approval.PassRuleContext{ApprovedCount: 0, RejectedCount: 1, TotalCount: 3}, approval.PassRulePending},
		{"EmptyTasks", approval.PassRuleContext{ApprovedCount: 0, RejectedCount: 0, TotalCount: 0}, approval.PassRulePending},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, s.Evaluate(tt.ctx), "Evaluate result should match expected")
		})
	}
}

// TestRatioPassStrategy tests ratio pass strategy scenarios.
func TestRatioPassStrategy(t *testing.T) {
	s := NewRatioPassStrategy()
	assert.Equal(t, approval.PassRatio, s.Rule(), "Rule should be PassRatio")

	tests := []struct {
		name     string
		ctx      approval.PassRuleContext
		expected approval.PassRuleResult
	}{
		{
			"ReachedRatio",
			approval.PassRuleContext{ApprovedCount: 3, RejectedCount: 0, TotalCount: 4, PassRatio: 60.0},
			approval.PassRulePassed,
		},
		{
			"ExactBoundary",
			approval.PassRuleContext{ApprovedCount: 2, RejectedCount: 0, TotalCount: 4, PassRatio: 50.0},
			approval.PassRulePassed,
		},
		{
			"BelowRatio",
			approval.PassRuleContext{ApprovedCount: 1, RejectedCount: 0, TotalCount: 4, PassRatio: 60.0},
			approval.PassRulePending,
		},
		{
			"ImpossibleToReach",
			approval.PassRuleContext{ApprovedCount: 0, RejectedCount: 3, TotalCount: 4, PassRatio: 50.0},
			approval.PassRuleRejected,
		},
		{
			"EmptyTasks",
			approval.PassRuleContext{ApprovedCount: 0, RejectedCount: 0, TotalCount: 0, PassRatio: 60.0},
			approval.PassRulePending,
		},
		{
			"HundredPercentAllApproved",
			approval.PassRuleContext{ApprovedCount: 3, RejectedCount: 0, TotalCount: 3, PassRatio: 100.0},
			approval.PassRulePassed,
		},
		{
			"HundredPercentOneRejected",
			approval.PassRuleContext{ApprovedCount: 2, RejectedCount: 1, TotalCount: 3, PassRatio: 100.0},
			approval.PassRuleRejected,
		},
		{
			"MaxRatioEqualsPassRatio",
			approval.PassRuleContext{ApprovedCount: 0, RejectedCount: 2, TotalCount: 4, PassRatio: 50.0},
			approval.PassRulePending,
		},
		{
			"ZeroPassRatio",
			approval.PassRuleContext{ApprovedCount: 0, RejectedCount: 0, TotalCount: 3, PassRatio: 0},
			approval.PassRulePassed,
		},
		{
			"FloatingPointPrecision",
			approval.PassRuleContext{ApprovedCount: 2, RejectedCount: 0, TotalCount: 3, PassRatio: 66.67},
			approval.PassRulePending,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, s.Evaluate(tt.ctx), "Evaluate result should match expected")
		})
	}
}

// TestOneRejectStrategy tests one reject strategy scenarios.
func TestOneRejectStrategy(t *testing.T) {
	s := NewOneRejectStrategy()
	assert.Equal(t, approval.PassAnyReject, s.Rule(), "Rule should be PassAnyReject")

	tests := []struct {
		name     string
		ctx      approval.PassRuleContext
		expected approval.PassRuleResult
	}{
		{"OneRejected", approval.PassRuleContext{ApprovedCount: 2, RejectedCount: 1, TotalCount: 3}, approval.PassRuleRejected},
		{"AllApproved", approval.PassRuleContext{ApprovedCount: 3, RejectedCount: 0, TotalCount: 3}, approval.PassRulePassed},
		{"PartialApproved", approval.PassRuleContext{ApprovedCount: 1, RejectedCount: 0, TotalCount: 3}, approval.PassRulePending},
		{"EmptyTasks", approval.PassRuleContext{ApprovedCount: 0, RejectedCount: 0, TotalCount: 0}, approval.PassRulePending},
		{"FirstRejected", approval.PassRuleContext{ApprovedCount: 0, RejectedCount: 1, TotalCount: 3}, approval.PassRuleRejected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, s.Evaluate(tt.ctx), "Evaluate result should match expected")
		})
	}
}
