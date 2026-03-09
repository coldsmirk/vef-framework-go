package js

import _ "embed"

// Embedded JavaScript library sources.
var (
	//go:embed libs/day.v1_11_19.js
	dayJs []byte
	//go:embed libs/big.v7_0_1.js
	bigJs []byte
	//go:embed libs/utils.v12_7_0.js
	utilsJs []byte
	//go:embed libs/validator.v13_15_20.js
	validatorJs []byte
)

// Pre-compiled library programs for efficient runtime initialization.
var (
	compiledDayJs       = MustCompile("day.js", string(dayJs), true)
	compiledBigJs       = MustCompile("big.js", string(bigJs), true)
	compiledUtilsJs     = MustCompile("utils.js", string(utilsJs), true)
	compiledValidatorJs = MustCompile("validator.js", string(validatorJs), true)
)
