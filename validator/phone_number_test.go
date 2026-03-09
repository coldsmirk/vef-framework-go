package validator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPhoneNumberValidation tests phone number validation functionality.
func TestPhoneNumberValidation(t *testing.T) {
	type testStruct struct {
		PhoneNumber string `validate:"phone_number" label:"手机号"`
	}

	tests := []struct {
		name    string
		phone   string
		wantErr bool
	}{
		{"ValidPhoneStartsWith13", "13888888888", false},
		{"ValidPhoneStartsWith15", "15999999999", false},
		{"ValidPhoneStartsWith18", "18666666666", false},
		{"ValidPhoneStartsWith19", "19777777777", false},
		{"ValidPhoneStartsWith14", "14888888888", false},
		{"ValidPhoneStartsWith16", "16888888888", false},
		{"ValidPhoneStartsWith17", "17888888888", false},
		{"ValidPhoneAllSameDigits", "13333333333", false},
		{"InvalidStartsWith12", "12888888888", true},
		{"InvalidStartsWith10", "10888888888", true},
		{"InvalidStartsWith0", "01888888888", true},
		{"InvalidStartsWith2", "21888888888", true},
		{"InvalidTooShort", "1388888888", true},
		{"InvalidTooLong", "138888888888", true},
		{"InvalidContainsLetters", "13888888a88", true},
		{"InvalidContainsSpecialChars", "1388888-888", true},
		{"InvalidEmptyString", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStruct{PhoneNumber: tt.phone}

			err := Validate(&s)
			if tt.wantErr {
				assert.Error(t, err, "Should return validation error for phone: %q", tt.phone)
				assert.Contains(t, err.Error(), "手机号", "Error message should contain label")
			} else {
				assert.NoError(t, err, "Should not return validation error for phone: %q", tt.phone)
			}
		})
	}
}
