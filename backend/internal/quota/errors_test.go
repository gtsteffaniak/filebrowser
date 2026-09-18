package quota

import "testing"

func TestErrorDisplayMessage(t *testing.T) {
	exceeded := newError(CodeExceeded, "folder", "uuid-id", 100, 90, 5, "")
	if exceeded.DisplayMessage() != "Folder storage quota exceeded" {
		t.Fatalf("folder exceeded: got %q", exceeded.DisplayMessage())
	}
	if exceeded.Error() != "Folder storage quota exceeded" {
		t.Fatalf("Error() should match DisplayMessage: got %q", exceeded.Error())
	}

	custom := newError(CodeExceeded, "folder", "uuid-id", 100, 90, 5, "Custom limit message")
	if custom.DisplayMessage() != "Custom limit message" {
		t.Fatalf("custom message: got %q", custom.DisplayMessage())
	}
}
