package security

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/password"
)

// PasswordHistoryStore persists a subject's previously used password hashes so a
// new password can be rejected if it repeats a recent one. The application owns
// the store (the user database belongs to it); the framework only reads history
// to check reuse and performs the hash comparison itself, so implementations
// never touch the password.Encoder.
type PasswordHistoryStore interface {
	// Recent returns the subject's most recent encoded passwords, newest first,
	// capped at limit.
	Recent(ctx context.Context, principalID string, limit int) ([]string, error)
	// Add records encodedPassword as the subject's newest history entry. Call it
	// from PasswordChanger.ChangePassword when a new password is persisted.
	Add(ctx context.Context, principalID, encodedPassword string) error
}

// NewChainValidator composes validators into one that returns the first
// violation, letting strength rules and history checks share a single
// PasswordValidator. Nil validators are skipped.
func NewChainValidator(validators ...PasswordValidator) PasswordValidator {
	return &chainValidator{validators: validators}
}

type chainValidator struct {
	validators []PasswordValidator
}

func (c *chainValidator) Validate(ctx context.Context, principal *Principal, plaintext string) error {
	for _, validator := range c.validators {
		if validator == nil {
			continue
		}

		if err := validator.Validate(ctx, principal, plaintext); err != nil {
			return err
		}
	}

	return nil
}

// NewHistoryValidator rejects a password that matches any of the subject's last
// depth entries. depth ≤ 0 disables the check. Comparison uses encoder because
// each stored hash carries its own salt.
func NewHistoryValidator(store PasswordHistoryStore, encoder password.Encoder, depth int) PasswordValidator {
	return &historyValidator{store: store, encoder: encoder, depth: depth}
}

type historyValidator struct {
	store   PasswordHistoryStore
	encoder password.Encoder
	depth   int
}

func (v *historyValidator) Validate(ctx context.Context, principal *Principal, plaintext string) error {
	if v.depth <= 0 || principal == nil {
		return nil
	}

	recent, err := v.store.Recent(ctx, principal.ID, v.depth)
	if err != nil {
		return err
	}

	for _, encoded := range recent {
		if v.encoder.Matches(plaintext, encoded) {
			return ErrPasswordReused
		}
	}

	return nil
}
