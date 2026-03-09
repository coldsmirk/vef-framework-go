package validator

import (
	"regexp"

	v "github.com/go-playground/validator/v10"
)

var (
	alphanumUsRegex      = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	alphanumUsSlashRegex = regexp.MustCompile(`^[a-zA-Z0-9_/]+$`)
	alphanumUsDotRegex   = regexp.MustCompile(`^[a-zA-Z0-9_.]+$`)
)

func newAlphanumUsRule() ValidationRule {
	return newRegexRule("alphanum_us", alphanumUsRegex, "{0}只能包含字母、数字和下划线", "validator_alphanum_us")
}

func newAlphanumUsSlashRule() ValidationRule {
	return newRegexRule("alphanum_us_slash", alphanumUsSlashRegex, "{0}只能包含字母、数字、下划线和斜线", "validator_alphanum_us_slash")
}

func newAlphanumUsDotRule() ValidationRule {
	return newRegexRule("alphanum_us_dot", alphanumUsDotRegex, "{0}只能包含字母、数字、下划线和点", "validator_alphanum_us_dot")
}

func newRegexRule(ruleTag string, regex *regexp.Regexp, errMessageTemplate, errMessageI18nKey string) ValidationRule {
	return ValidationRule{
		RuleTag:                  ruleTag,
		ErrMessageTemplate:       errMessageTemplate,
		ErrMessageI18nKey:        errMessageI18nKey,
		CallValidationEvenIfNull: false,
		Validate: func(fl v.FieldLevel) bool {
			return regex.MatchString(fl.Field().String())
		},
		ParseParam: func(fe v.FieldError) []string {
			return []string{fe.Field()}
		},
	}
}
