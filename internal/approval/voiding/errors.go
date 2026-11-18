package voiding

import "errors"

var errMissingPlan = errors.New("business voider returned a nil plan")
