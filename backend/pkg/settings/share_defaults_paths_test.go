package settings

import "testing"

func TestShareDefaultsValueAtPath_includesZeroBool(t *testing.T) {
	d := ShareDefaults{AllowModify: false}
	val, ok := ShareDefaultsValueAtPath(d, "allowModify")
	if !ok {
		t.Fatal("expected allowModify path")
	}
	if val != false {
		t.Fatalf("allowModify=%v want false", val)
	}
}
