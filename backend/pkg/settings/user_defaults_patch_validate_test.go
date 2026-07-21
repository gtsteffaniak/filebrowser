package settings

import "testing"

func TestValidateSinglePropertyUserDefaultsPatch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		patch   string
		wantErr bool
	}{
		{"one bool", `{"listing":{"showHidden":true}}`, false},
		{"one string", `{"listing":{"hideFileExt":".exe"}}`, false},
		{"enforced shape", `{"account":{"lockPassword":true}}`, false},
		{"two fields", `{"listing":{"showHidden":true,"singleClick":true}}`, true},
		{"two sections", `{"listing":{"showHidden":true},"sidebar":{"sticky":true}}`, true},
		{"empty", `{}`, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateSinglePropertyUserDefaultsPatch([]byte(tc.patch))
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
