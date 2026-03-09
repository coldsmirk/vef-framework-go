package validator

import "regexp"

var phoneNumberRegex = regexp.MustCompile(`^1[3-9]\d{9}$`)

func newPhoneNumberRule() ValidationRule {
	return newRegexRule("phone_number", phoneNumberRegex, "{0}格式不正确", "validator_phone_number")
}
