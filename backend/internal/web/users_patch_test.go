package web

import "testing"

func TestValidatePatchWhich(t *testing.T) {
	t.Run("accepts frontend fields", func(t *testing.T) {
		for _, which := range [][]string{
			{"scopes", "permissions"},
			{"locale"},
			{"loginMethod", "otpEnabled"},
			{"password"},
		} {
			if err := validatePatchWhich(which); err != nil {
				t.Fatalf("unexpected error for which=%v: %v", which, err)
			}
		}
	})

	t.Run("rejects empty", func(t *testing.T) {
		if err := validatePatchWhich(nil); err == nil {
			t.Fatal("expected error for nil which")
		}
		if err := validatePatchWhich([]string{}); err == nil {
			t.Fatal("expected error for empty which")
		}
	})

	t.Run("rejects invalid field names", func(t *testing.T) {
		for _, which := range [][]string{{"all"}, {"All"}, {"scopes", "all"}} {
			if err := validatePatchWhich(which); err == nil {
				t.Fatalf("expected error for which=%v", which)
			}
		}
	})

	t.Run("rejects blank field names", func(t *testing.T) {
		if err := validatePatchWhich([]string{"scopes", " "}); err == nil {
			t.Fatal("expected error for blank field name")
		}
	})

	t.Run("rejects unknown field names", func(t *testing.T) {
		if err := validatePatchWhich([]string{"notARealField"}); err == nil {
			t.Fatal("expected error for unknown field name")
		}
	})

	t.Run("rejects backend user fields", func(t *testing.T) {
		for _, field := range []string{"id", "tokens", "backendScopes", "version", "totpSecret", "pinnedItems"} {
			if err := validatePatchWhich([]string{field}); err == nil {
				t.Fatalf("expected error for backend field %q", field)
			}
		}
	})
}
