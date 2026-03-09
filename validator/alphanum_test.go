package validator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAlphanumUs tests alphanum us functionality.
func TestAlphanumUs(t *testing.T) {
	type testStruct struct {
		Value string `validate:"alphanum_us"`
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"LowercaseLetters", "abc", false},
		{"UppercaseLetters", "ABC", false},
		{"MixedCase", "AbCdEf", false},
		{"numbers", "123", false},
		{"Alphanumeric", "abc123", false},
		{"WithUnderscores", "abc_123", false},
		{"MultipleUnderscores", "abc__123__def", false},
		{"LeadingUnderscore", "_abc", false},
		{"TrailingUnderscore", "abc_", false},
		{"OnlyUnderscores", "___", false},
		{"SnakeCase", "get_user_info", false},
		{"WithSpace", "abc 123", true},
		{"WithDash", "abc-123", true},
		{"WithSlash", "abc/123", true},
		{"WithDot", "abc.123", true},
		{"WithSpecialChars", "abc@123", true},
		{"EmptyString", "", true},
		{"ChineseCharacters", "中文", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStruct{Value: tt.value}

			err := Validate(&s)
			if tt.wantErr {
				assert.Error(t, err, "Should return validation error for value: %q", tt.value)
			} else {
				assert.NoError(t, err, "Should not return validation error for value: %q", tt.value)
			}
		})
	}
}

// TestAlphanumUsSlash tests alphanum us slash functionality.
func TestAlphanumUsSlash(t *testing.T) {
	type testStruct struct {
		Value string `validate:"alphanum_us_slash"`
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"LowercaseLetters", "abc", false},
		{"UppercaseLetters", "ABC", false},
		{"numbers", "123", false},
		{"Alphanumeric", "abc123", false},
		{"WithUnderscores", "abc_123", false},
		{"WithSlashes", "abc/123", false},
		{"ResourcePath", "sys/user", false},
		{"NestedPath", "auth/get_user_info", false},
		{"MultipleSlashes", "a/b/c/d", false},
		{"LeadingSlash", "/abc", false},
		{"TrailingSlash", "abc/", false},
		{"mixed", "sys_module/user_info", false},
		{"WithSpace", "abc 123", true},
		{"WithDash", "abc-123", true},
		{"WithDot", "abc.123", true},
		{"WithSpecialChars", "abc@123", true},
		{"EmptyString", "", true},
		{"ChineseCharacters", "中文", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStruct{Value: tt.value}

			err := Validate(&s)
			if tt.wantErr {
				assert.Error(t, err, "Should return validation error for value: %q", tt.value)
			} else {
				assert.NoError(t, err, "Should not return validation error for value: %q", tt.value)
			}
		})
	}
}

// TestAlphanumUsDot tests alphanum us dot functionality.
func TestAlphanumUsDot(t *testing.T) {
	type testStruct struct {
		Value string `validate:"alphanum_us_dot"`
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"LowercaseLetters", "abc", false},
		{"UppercaseLetters", "ABC", false},
		{"numbers", "123", false},
		{"Alphanumeric", "abc123", false},
		{"WithUnderscores", "abc_123", false},
		{"WithDots", "abc.123", false},
		{"FileName", "config.yaml", false},
		{"ModuleName", "com.example.app", false},
		{"VersionNumber", "v1.2.3", false},
		{"MultipleDots", "a.b.c.d", false},
		{"LeadingDot", ".hidden", false},
		{"TrailingDot", "file.", false},
		{"mixed", "app_config.prod.yaml", false},
		{"WithSpace", "abc 123", true},
		{"WithDash", "abc-123", true},
		{"WithSlash", "abc/123", true},
		{"WithSpecialChars", "abc@123", true},
		{"EmptyString", "", true},
		{"ChineseCharacters", "中文", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStruct{Value: tt.value}

			err := Validate(&s)
			if tt.wantErr {
				assert.Error(t, err, "Should return validation error for value: %q", tt.value)
			} else {
				assert.NoError(t, err, "Should not return validation error for value: %q", tt.value)
			}
		})
	}
}

// TestAlphanumRulesCombined tests alphanum rules combined functionality.
func TestAlphanumRulesCombined(t *testing.T) {
	type testStruct struct {
		Action   string `validate:"alphanum_us" label:"操作"`
		Resource string `validate:"alphanum_us_slash" label:"资源"`
		FileName string `validate:"alphanum_us_dot" label:"文件名"`
	}

	tests := []struct {
		name     string
		action   string
		resource string
		fileName string
		wantErr  bool
	}{
		{
			name:     "AllValid",
			action:   "get_user_info",
			resource: "sys/user",
			fileName: "config.yaml",
			wantErr:  false,
		},
		{
			name:     "InvalidActionWithSlash",
			action:   "get/user",
			resource: "sys/user",
			fileName: "config.yaml",
			wantErr:  true,
		},
		{
			name:     "InvalidResourceWithDot",
			action:   "get_user",
			resource: "sys.user",
			fileName: "config.yaml",
			wantErr:  true,
		},
		{
			name:     "InvalidFilenameWithSlash",
			action:   "get_user",
			resource: "sys/user",
			fileName: "config/app.yaml",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStruct{
				Action:   tt.action,
				Resource: tt.resource,
				FileName: tt.fileName,
			}

			err := Validate(&s)
			if tt.wantErr {
				assert.Error(t, err, "Should return validation error")
			} else {
				assert.NoError(t, err, "Should not return validation error")
			}
		})
	}
}
