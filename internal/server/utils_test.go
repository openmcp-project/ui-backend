package server

import (
	"testing"
)

func TestParseAuthorizationHeaderWithDoubleTokens(t *testing.T) {
	tests := []struct {
		authHeader string
		token1     string
		token2     string
		expectErr  bool
	}{
		{"token1,token2", "token1", "token2", false},
		{"token1", "token1", "", false},
		{"", "", "", true},
		{"token1,token2,token3", "", "", true},
	}

	for _, test := range tests {
		t.Run(test.authHeader, func(t *testing.T) {
			token1, token2, err := parseAuthorizationHeaderWithDoubleTokens(test.authHeader)

			if test.expectErr {
				if err == nil {
					t.Errorf("expected an error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
				if token1 != test.token1 {
					t.Errorf("expected token1 to be %q but got %q", test.token1, token1)
				}
				if token2 != test.token2 {
					t.Errorf("expected token2 to be %q but got %q", test.token2, token2)
				}
			}
		})
	}
}
