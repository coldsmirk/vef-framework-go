package security

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/coldsmirk/go-collections"
)

// PasswordValidator checks a candidate plaintext password against a policy. The
// framework builds one from configuration and injects it; applications can also
// call it directly from their own registration or reset flows.
type PasswordValidator interface {
	// Validate returns a non-nil error (a result.Error carrying an i18n message
	// and a 400 status) describing the first policy violation, or nil when the
	// password is acceptable. The principal enables context-aware rules such as
	// rejecting a password that echoes the account identity.
	Validate(ctx context.Context, principal *Principal, plaintext string) error
}

// PasswordRule is a single composable password-policy constraint.
type PasswordRule interface {
	// Check returns a non-nil error when plaintext violates the rule, or nil
	// when it satisfies the rule.
	Check(principal *Principal, plaintext string) error
}

// NewRuleBasedValidator composes rules into a PasswordValidator that returns the
// first violation. With no rules it accepts every password, so an unconfigured
// policy imposes no constraint.
func NewRuleBasedValidator(rules ...PasswordRule) PasswordValidator {
	return &ruleBasedValidator{rules: rules}
}

type ruleBasedValidator struct {
	rules []PasswordRule
}

func (v *ruleBasedValidator) Validate(_ context.Context, principal *Principal, plaintext string) error {
	for _, rule := range v.rules {
		if err := rule.Check(principal, plaintext); err != nil {
			return err
		}
	}

	return nil
}

// NewMinLengthRule requires at least minLength characters (counted as runes).
func NewMinLengthRule(minLength int) PasswordRule {
	return &minLengthRule{minLength: minLength}
}

type minLengthRule struct {
	minLength int
}

func (r *minLengthRule) Check(_ *Principal, plaintext string) error {
	if utf8.RuneCountInString(plaintext) < r.minLength {
		return ErrPasswordTooShort(r.minLength)
	}

	return nil
}

// NewMaxLengthRule requires at most maxLength characters (counted as runes). A
// bound guards against slow-KDF denial of service and silent bcrypt truncation.
func NewMaxLengthRule(maxLength int) PasswordRule {
	return &maxLengthRule{maxLength: maxLength}
}

type maxLengthRule struct {
	maxLength int
}

func (r *maxLengthRule) Check(_ *Principal, plaintext string) error {
	if utf8.RuneCountInString(plaintext) > r.maxLength {
		return ErrPasswordTooLong(r.maxLength)
	}

	return nil
}

// NewCharacterClassRule enforces required character classes and, when
// minClasses > 0, a minimum count of distinct classes present. The four classes
// are uppercase, lowercase, digit, and symbol (any non-space, non-letter,
// non-digit rune); caseless letters such as CJK count toward no class.
func NewCharacterClassRule(requireUpper, requireLower, requireDigit, requireSymbol bool, minClasses int) PasswordRule {
	return &characterClassRule{
		requireUpper:  requireUpper,
		requireLower:  requireLower,
		requireDigit:  requireDigit,
		requireSymbol: requireSymbol,
		minClasses:    minClasses,
	}
}

type characterClassRule struct {
	requireUpper  bool
	requireLower  bool
	requireDigit  bool
	requireSymbol bool
	minClasses    int
}

func (r *characterClassRule) Check(_ *Principal, plaintext string) error {
	var hasUpper, hasLower, hasDigit, hasSymbol bool
	for _, c := range plaintext {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		case !unicode.IsLetter(c) && !unicode.IsSpace(c):
			// Caseless letters (e.g. CJK) are excluded: they are not symbols and
			// must not satisfy a require_symbol or min-classes policy.
			hasSymbol = true
		}
	}

	switch {
	case r.requireUpper && !hasUpper:
		return ErrPasswordMissingUppercase
	case r.requireLower && !hasLower:
		return ErrPasswordMissingLowercase
	case r.requireDigit && !hasDigit:
		return ErrPasswordMissingDigit
	case r.requireSymbol && !hasSymbol:
		return ErrPasswordMissingSymbol
	}

	if r.minClasses > 0 {
		classes := 0
		for _, present := range []bool{hasUpper, hasLower, hasDigit, hasSymbol} {
			if present {
				classes++
			}
		}

		if classes < r.minClasses {
			return ErrPasswordTooFewCharClasses(r.minClasses)
		}
	}

	return nil
}

// identityMinToken is the shortest identity fragment (in runes) the
// disallow-identity rule will match on, so a two-character name cannot reject
// most passwords.
const identityMinToken = 3

// NewDisallowIdentityRule rejects a password that contains the principal's login
// id or display name (case-insensitive), the most common weak-password pattern.
func NewDisallowIdentityRule() PasswordRule {
	return new(disallowIdentityRule)
}

type disallowIdentityRule struct{}

func (*disallowIdentityRule) Check(principal *Principal, plaintext string) error {
	if principal == nil {
		return nil
	}

	lowered := strings.ToLower(plaintext)
	for _, token := range []string{principal.ID, principal.Name} {
		token = strings.ToLower(strings.TrimSpace(token))
		if utf8.RuneCountInString(token) >= identityMinToken && strings.Contains(lowered, token) {
			return ErrPasswordContainsIdentity
		}
	}

	return nil
}

// NewBlocklistRule rejects passwords that match any blocked entry
// (case-insensitive). Entries are compared verbatim after trimming; supply a
// deny list of well-known weak passwords for the deployment.
func NewBlocklistRule(entries []string) PasswordRule {
	lowered := make([]string, 0, len(entries))
	for _, entry := range entries {
		if trimmed := strings.ToLower(strings.TrimSpace(entry)); trimmed != "" {
			lowered = append(lowered, trimmed)
		}
	}

	return &blocklistRule{blocked: collections.NewHashSetFrom(lowered...)}
}

type blocklistRule struct {
	blocked collections.Set[string]
}

func (r *blocklistRule) Check(_ *Principal, plaintext string) error {
	if r.blocked.Contains(strings.ToLower(strings.TrimSpace(plaintext))) {
		return ErrPasswordBlocked
	}

	return nil
}
