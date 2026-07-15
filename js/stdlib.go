package js

import _ "embed"

// Embedded standard library sources.
var (
	//go:embed libs/day.v1_11_19.js
	dayJsSource []byte
	//go:embed libs/big.v7_0_1.js
	bigJsSource []byte
	//go:embed libs/utils.v12_7_0.js
	utilsJsSource []byte
	//go:embed libs/validator.v13_15_20.js
	validatorJsSource []byte
)

// stdLibs are the pure JavaScript standard libraries installed into every
// runtime unless the engine was built with WithoutStdLibs. They are named
// after the global binding each one installs.
var stdLibs = []Lib{
	ProgramLib("dayjs", MustCompile("dayjs", string(dayJsSource), true)),
	ProgramLib("Big", MustCompile("Big", string(bigJsSource), true)),
	ProgramLib("utils", MustCompile("utils", string(utilsJsSource), true)),
	ProgramLib("validator", MustCompile("validator", string(validatorJsSource), true)),
}
