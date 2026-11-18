package voiding

import (
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
)

var errInvalidVoidInput = result.Err(i18n.T("approval_void_input_invalid"), result.WithCode(result.ErrCodeBadRequest))
