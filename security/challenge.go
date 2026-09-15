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

// LoginContext is the login a challenge runs within: the mechanism that
// authenticated the principal, the identifier presented, and the challenges
// already behind it. It is what lets a challenge apply per login mechanism — a
// forced password change on password logins but not on a host-defined
// mini-program login — instead of to every login alike.
//
// The hooks behind a challenge divide along one rule. A hook that decides
// whether a challenge applies (PasswordChangeChecker, OTPEvaluator,
// DepartmentLoader) sees the login; a hook that acts on the identity
// (PasswordChanger, OTPCodeSender, OTPCodeVerifier, DepartmentSelector, …) sees
// only the principal, since how the user logged in has no bearing on how a
// password is stored or a code is checked.
//
// The framework owns it and passes it by pointer: treat it as read-only. A
// provider changes the identity by returning a principal from Resolve, never by
// writing to the context.
type LoginContext struct {
	// AuthType is the login mechanism that authenticated the principal —
	// AuthTypePassword, AuthTypeTrustCode, or a host-defined type. It stays the
	// same across every challenge step of one login.
	AuthType string
	// Username is the identifier presented at the first login step, so audit
	// events raised after a challenge report the same identifier as the initial
	// login, independent of the principal's display name.
	Username string
	// Principal is the identity as enriched by the challenges resolved so far.
	Principal *Principal
	// Resolved lists the challenge types resolved so far, in resolution order.
	Resolved []string
}

// ChallengeState is what a challenge token carries between login steps: the
// login so far and the challenge types still ahead of it.
type ChallengeState struct {
	LoginContext

	// Pending lists the challenge types not yet resolved, in evaluation order.
	// The first is the challenge presented to the user.
	Pending []string
}

// ChallengeTokenStore manages the lifecycle of challenge tokens.
// Challenge tokens carry the intermediate state between login steps,
// allowing the login flow to pause for user input (e.g., 2FA code, department selection).
// The default implementation uses JWT; alternatives (e.g., Redis) can be swapped via DI.
//
// Generate and Parse must round-trip every field of the state. The login flow
// refuses a parsed state without an AuthType exactly as it refuses a token that
// does not parse, since the challenges still ahead are scoped by it.
type ChallengeTokenStore interface {
	// Generate creates a challenge token carrying state. ctx lets I/O-backed
	// implementations honor deadlines, cancellation, and trace propagation.
	Generate(ctx context.Context, state *ChallengeState) (string, error)
	// Parse retrieves the challenge state a token carries. The caller owns the
	// returned state: the login flow reslices Pending, appends to Resolved and
	// replaces Principal, then hands the state to Generate — so its slices must
	// not share backing arrays with anything the store retains.
	Parse(ctx context.Context, token string) (*ChallengeState, error)
}

// ChallengeProvider evaluates and resolves a login challenge.
// Register implementations via vef.ProvideChallengeProvider to inject
// additional steps into the login flow (e.g., 2FA, department selection).
//
// Providers are evaluated sequentially in Order() ascending order.
// Each challenge is presented and resolved one at a time before
// the next provider is evaluated.
//
// A provider applies to every login unless it decides otherwise from the login
// it is handed; to scope one to some login mechanisms without touching it, wrap
// it with NewFilteredChallengeProvider.
type ChallengeProvider interface {
	// Type returns the unique challenge type identifier (e.g. "totp", "department_selection").
	Type() string
	// Order returns the evaluation priority. Lower values are evaluated first.
	Order() int
	// Evaluate checks whether this challenge applies to the login.
	// Return nil to indicate the challenge is not needed.
	Evaluate(ctx context.Context, login *LoginContext) (*LoginChallenge, error)
	// Resolve validates the user's response and returns the principal the login
	// continues with — login.Principal, or an enriched copy of it.
	Resolve(ctx context.Context, login *LoginContext, response any) (*Principal, error)
}
