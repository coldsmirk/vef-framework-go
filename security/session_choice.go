package security

import "context"

// SessionChoiceProvider is a ChallengeProvider whose answer is a choice the
// session is built on — the department the user acts in — rather than a proof
// checked once and forgotten, like a one-time code.
//
// Such a choice must outlive the login. A JWT refresh rebuilds the principal
// from UserLoader.LoadByID, which knows the user but not what the user chose
// when logging in, so without help every refresh would quietly fall back to
// whatever the user record alone yields — a default department in place of the
// one picked. The login flow therefore records SessionChoice of every resolved
// answer in Principal.SessionChoices, the refresh token carries them, and a
// refresh hands each back to ReapplyChoice on the reloaded principal.
//
// Only an answer that names a choice belongs here: a credential or a proof never
// does, since the refresh token would carry it for the life of the session.
type SessionChoiceProvider interface {
	ChallengeProvider
	// SessionChoice returns what of a response Resolve accepted the session
	// keeps, and false when it keeps nothing.
	SessionChoice(response any) (string, bool)
	// ReapplyChoice applies a choice recorded at login to a principal freshly
	// loaded on refresh, validating it again: it returns the principal the
	// session continues with, or an error when the choice no longer holds — the
	// refresh then fails and the user logs in to choose again.
	ReapplyChoice(ctx context.Context, principal *Principal, choice string) (*Principal, error)
}
