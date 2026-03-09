package security

import "context"

// LoginChallenge describes a challenge the user must complete during login.
type LoginChallenge struct {
	Type     string `json:"type"`
	Data     any    `json:"data,omitempty"`
	Required bool   `json:"required"`
}

// LoginResult represents the response of a login attempt.
// When a challenge is pending, Tokens is nil and ChallengeToken + Challenge are set.
// When all challenges are resolved (or none were needed), Tokens is set.
type LoginResult struct {
	Tokens         *AuthTokens     `json:"tokens,omitempty"`
	ChallengeToken string          `json:"challengeToken,omitempty"`
	Challenge      *LoginChallenge `json:"challenge,omitempty"`
}

// ChallengeState holds the state tracked by a challenge token.
type ChallengeState struct {
	Principal *Principal
	Pending   []string
	Resolved  []string
}

// ChallengeTokenStore manages the lifecycle of challenge tokens.
// Challenge tokens carry the intermediate state between login steps,
// allowing the login flow to pause for user input (e.g., 2FA code, department selection).
// The default implementation uses JWT; alternatives (e.g., Redis) can be swapped via DI.
type ChallengeTokenStore interface {
	// Generate creates a challenge token encoding the principal and challenge state.
	Generate(principal *Principal, pending, resolved []string) (string, error)
	// Parse retrieves the challenge state from a token.
	Parse(token string) (*ChallengeState, error)
}

// ChallengeProvider evaluates and resolves a login challenge.
// Register implementations via vef.ProvideChallengeProvider to inject
// additional steps into the login flow (e.g., 2FA, department selection).
//
// Providers are evaluated sequentially in Order() ascending order.
// Each challenge is presented and resolved one at a time before
// the next provider is evaluated.
type ChallengeProvider interface {
	// Type returns the unique challenge type identifier (e.g. "totp", "select_department").
	Type() string
	// Order returns the evaluation priority. Lower values are evaluated first.
	Order() int
	// Evaluate checks whether this challenge applies to the given principal.
	// Return nil to indicate the challenge is not needed.
	Evaluate(ctx context.Context, principal *Principal) (*LoginChallenge, error)
	// Resolve validates the user's response and returns an optionally updated Principal.
	Resolve(ctx context.Context, principal *Principal, response any) (*Principal, error)
}
