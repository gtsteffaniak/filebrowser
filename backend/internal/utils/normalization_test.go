package utils

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/unicode/norm"
)

// nfdName is "blää.txt" with the umlauts in decomposed (NFD) form — the byte
// sequence macOS browsers send for uploads.
const nfdName = "bla\u0308a\u0308.txt"

func TestNormalizeFinalSegment(t *testing.T) {
	nfcName := norm.NFC.String(nfdName)
	if nfcName == nfdName {
		t.Fatal("test fixture must be decomposed form")
	}

	cases := []struct {
		name, in, form, want string
	}{
		{"nfc form converts NFD filename to NFC", "/uploads/" + nfdName, "nfc", "/uploads/" + nfcName},
		{"nfd form converts NFC filename to NFD", "/uploads/" + nfcName, "nfd", "/uploads/" + nfdName},
		{"none form is a no-op", "/uploads/" + nfdName, "none", "/uploads/" + nfdName},
		{"unset form is a no-op", "/uploads/" + nfdName, "", "/uploads/" + nfdName},
		{"unknown form is a no-op", "/uploads/" + nfdName, "NFKC", "/uploads/" + nfdName},
		{"parent segment is never rewritten", "/" + nfdName + "/readme.txt", "nfc", "/" + nfdName + "/readme.txt"},
		{"root is untouched", "/", "nfc", "/"},
		{"ascii-only names are unchanged", "/plain/name.txt", "nfc", "/plain/name.txt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeFinalSegment(tc.in, tc.form); got != tc.want {
				t.Errorf("NormalizeFinalSegment(%q, %q) = %q, want %q", tc.in, tc.form, got, tc.want)
			}
		})
	}
}

// TestNormalizeFinalSegmentRoundTripOnDisk pins the original motivation
// (#2823): with nfc configured, a name that arrives decomposed must land on
// disk composed, byte-for-byte — which is what the macOS SMB client asks for.
func TestNormalizeFinalSegmentRoundTripOnDisk(t *testing.T) {
	dir := t.TempDir()
	onDisk := filepath.Join(dir, NormalizeFinalSegment(nfdName, "nfc"))

	if err := os.WriteFile(onDisk, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one entry, got %d", len(entries))
	}
	if !norm.NFC.IsNormalString(entries[0].Name()) {
		t.Errorf("on-disk name %q is not NFC", entries[0].Name())
	}
}
