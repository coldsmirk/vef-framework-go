package service

import (
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
)

var errAnnotationContent = result.Err(i18n.T("approval_annotation_content_invalid"), result.WithCode(result.ErrCodeBadRequest))
