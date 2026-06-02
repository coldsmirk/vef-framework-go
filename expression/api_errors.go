package expression

import (
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
)

// Response codes for expression API errors (2500-2599).
const ErrCodeEvaluationFailed = 2500

// ErrEvaluationFailed is the outward error for any expression compile or
// evaluation failure. Backends wrap their cause with it (via errors join) so
// the stable code drives API mapping while the cause stays available for logs.
var ErrEvaluationFailed = result.Err(
	i18n.T("expression_evaluation_failed"),
	result.WithCode(ErrCodeEvaluationFailed),
)
