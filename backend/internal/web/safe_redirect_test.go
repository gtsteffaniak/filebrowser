package web

import "testing"

func TestSafeRelativeRedirectPath(t *testing.T) {
	tests := []struct {
		raw  string
		want string
		ok   bool
	}{
		{"/files/", "/files/", true},
		{"/files/?q=1", "/files/?q=1", true},
		{"", "", false},
		{"files/", "", false},
		{"//evil.example/", "", false},
		{"/\\evil.example/", "", false},
		{"https://evil.example/", "", false},
		{"/files/\n", "", false},
		{"%2Ffiles%2F", "/files/", true},
		{"/files/?q=a+b", "/files/?q=a+b", true},
	}
	for _, tt := range tests {
		got, ok := SafeRelativeRedirectPath(tt.raw)
		if ok != tt.ok || got != tt.want {
			t.Errorf("SafeRelativeRedirectPath(%q) = (%q, %v), want (%q, %v)", tt.raw, got, ok, tt.want, tt.ok)
		}
	}
}
