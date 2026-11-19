package command

import (
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
)

var errInvalidIdempotencyKey = result.Err(i18n.T("approval_idempotency_key_invalid"), result.WithCode(result.ErrCodeBadRequest))
