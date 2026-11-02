package web

import "testing"

func TestEmbeddedFrontendAvailable_placeholderOnly(t *testing.T) {
	if EmbeddedFrontendAvailable() {
		t.Fatal("expected false when dist embed only contains the placeholder")
	}
}
