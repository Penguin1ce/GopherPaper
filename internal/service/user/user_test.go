package user

import "testing"

func TestLoginLookup(t *testing.T) {
	tests := []struct {
		name      string
		account   string
		wantQuery string
		wantArg   string
	}{
		{
			name:      "student id",
			account:   " 20260001 ",
			wantQuery: "student_id = ?",
			wantArg:   "20260001",
		},
		{
			name:      "email",
			account:   " student@example.com ",
			wantQuery: "email = ?",
			wantArg:   "student@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, arg := loginLookup(tt.account)
			if query != tt.wantQuery || arg != tt.wantArg {
				t.Fatalf("loginLookup() = (%q, %q), want (%q, %q)", query, arg, tt.wantQuery, tt.wantArg)
			}
		})
	}
}
