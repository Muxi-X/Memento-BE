package application

import "testing"

func TestIsValidVerificationCode(t *testing.T) {
	tests := []struct {
		code string
		want bool
	}{
		{code: "123456", want: true},
		{code: "12345", want: false},
		{code: "1234567", want: false},
		{code: "12a456", want: false},
		{code: "123 56", want: false},
		{code: "", want: false},
	}

	for _, tc := range tests {
		if got := isValidVerificationCode(tc.code); got != tc.want {
			t.Fatalf("isValidVerificationCode(%q) = %v, want %v", tc.code, got, tc.want)
		}
	}
}
