package security

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/result"
)

func assertPolicyViolation(t *testing.T, err error, msg string) {
	t.Helper()

	resErr, ok := result.AsErr(err)
	require.True(t, ok, msg)
	assert.Equal(t, ErrCodePasswordPolicyViolation, resErr.Code, "violation should carry the password-policy code")
}

func TestMinLengthRule(t *testing.T) {
	rule := NewMinLengthRule(8)

	t.Run("RejectsShort", func(t *testing.T) {
		assertPolicyViolation(t, rule.Check(nil, "short"), "a password below the minimum should be rejected")
	})

	t.Run("AcceptsExactLength", func(t *testing.T) {
		assert.NoError(t, rule.Check(nil, "exactly8"), "a password at the minimum length should pass")
	})

	t.Run("CountsRunesNotBytes", func(t *testing.T) {
		// Eight multibyte runes are 8 characters even though they are >8 bytes.
		assert.NoError(t, rule.Check(nil, "密码密码密码密码"), "length should be counted in runes")
	})
}

func TestMaxLengthRule(t *testing.T) {
	rule := NewMaxLengthRule(12)

	t.Run("RejectsLong", func(t *testing.T) {
		assertPolicyViolation(t, rule.Check(nil, "thispasswordistoolong"), "a password above the maximum should be rejected")
	})

	t.Run("AcceptsExactLength", func(t *testing.T) {
		assert.NoError(t, rule.Check(nil, "exactly12chr"), "a password at the maximum length should pass")
	})
}

func TestCharacterClassRule(t *testing.T) {
	t.Run("RequiredClasses", func(t *testing.T) {
		rule := NewCharacterClassRule(true, true, true, true, 0)

		tests := []struct {
			name     string
			password string
			ok       bool
		}{
			{"AllClassesPresent", "Abcd1234!", true},
			{"MissingUppercase", "abcd1234!", false},
			{"MissingLowercase", "ABCD1234!", false},
			{"MissingDigit", "Abcdefg!", false},
			{"MissingSymbol", "Abcd1234", false},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				err := rule.Check(nil, tc.password)
				if tc.ok {
					assert.NoError(t, err, "password satisfying all required classes should pass")
				} else {
					assertPolicyViolation(t, err, "password missing a required class should be rejected")
				}
			})
		}
	})

	t.Run("MinDistinctClasses", func(t *testing.T) {
		rule := NewCharacterClassRule(false, false, false, false, 3)

		assert.NoError(t, rule.Check(nil, "Abc123"), "three distinct classes should satisfy the minimum")
		assertPolicyViolation(t, rule.Check(nil, "abcdef"), "one class should not satisfy a minimum of three")
	})

	t.Run("WhitespaceIsNotASymbol", func(t *testing.T) {
		rule := NewCharacterClassRule(false, false, false, true, 0)

		assertPolicyViolation(t, rule.Check(nil, "abc def"), "whitespace must not count as a symbol")
		assert.NoError(t, rule.Check(nil, "abc!def"), "a punctuation mark should count as a symbol")
	})
}

func TestDisallowIdentityRule(t *testing.T) {
	rule := NewDisallowIdentityRule()
	principal := NewUser("alice001", "Alice")

	t.Run("RejectsPasswordContainingID", func(t *testing.T) {
		assertPolicyViolation(t, rule.Check(principal, "myalice001pw"), "a password containing the id should be rejected")
	})

	t.Run("RejectsPasswordContainingNameCaseInsensitive", func(t *testing.T) {
		assertPolicyViolation(t, rule.Check(principal, "xxALICExx"), "a password containing the name should be rejected regardless of case")
	})

	t.Run("AcceptsUnrelatedPassword", func(t *testing.T) {
		assert.NoError(t, rule.Check(principal, "unrelated-secret"), "a password without the identity should pass")
	})

	t.Run("IgnoresShortIdentityTokens", func(t *testing.T) {
		shortName := NewUser("ab", "Al")
		assert.NoError(t, rule.Check(shortName, "album-cover-ab"), "identity fragments shorter than the minimum must not match")
	})

	t.Run("NilPrincipalPasses", func(t *testing.T) {
		assert.NoError(t, rule.Check(nil, "anything"), "a nil principal should impose no identity constraint")
	})
}

func TestBlocklistRule(t *testing.T) {
	rule := NewBlocklistRule([]string{"password", "  Letmein  ", ""})

	t.Run("RejectsBlockedCaseInsensitive", func(t *testing.T) {
		assertPolicyViolation(t, rule.Check(nil, "PASSWORD"), "a blocked password should be rejected regardless of case")
	})

	t.Run("RejectsTrimmedEntry", func(t *testing.T) {
		assertPolicyViolation(t, rule.Check(nil, "letmein"), "entries should be trimmed before comparison")
	})

	t.Run("AcceptsAllowedPassword", func(t *testing.T) {
		assert.NoError(t, rule.Check(nil, "a-unique-secret"), "a non-blocked password should pass")
	})

	t.Run("BlankEntriesAreIgnored", func(t *testing.T) {
		assert.NoError(t, rule.Check(nil, ""), "an empty candidate should not match a discarded blank entry")
	})
}

func TestRuleBasedValidator(t *testing.T) {
	ctx := context.Background()
	principal := NewUser("u1", "Alice")

	t.Run("EmptyValidatorAcceptsAnything", func(t *testing.T) {
		validator := NewRuleBasedValidator()
		assert.NoError(t, validator.Validate(ctx, principal, "x"), "a validator with no rules should accept any password")
	})

	t.Run("ReturnsFirstViolationInOrder", func(t *testing.T) {
		validator := NewRuleBasedValidator(NewMinLengthRule(8), NewCharacterClassRule(true, false, false, false, 0))

		// "short" fails the length rule first, so that is the reported error even
		// though it also lacks an uppercase letter.
		err := validator.Validate(ctx, principal, "short")
		assertPolicyViolation(t, err, "the first failing rule should be reported")
		resErr, _ := result.AsErr(err)
		assert.Equal(t, ErrPasswordTooShort(8).Message, resErr.Message, "length rule should be evaluated before the class rule")
	})

	t.Run("PassesWhenAllRulesSatisfied", func(t *testing.T) {
		validator := NewRuleBasedValidator(NewMinLengthRule(8), NewCharacterClassRule(true, true, true, false, 0))
		assert.NoError(t, validator.Validate(ctx, principal, "Abcd1234"), "a password satisfying every rule should pass")
	})
}
