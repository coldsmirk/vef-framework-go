package calendar

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
)

var errInvalidDates = result.Err(i18n.T("calendar_invalid_dates"), result.WithCode(result.ErrCodeBadRequest), result.WithStatus(fiber.StatusBadRequest))
