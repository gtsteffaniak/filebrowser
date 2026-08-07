package activity

import "testing"

func TestParseActivityExportRows(t *testing.T) {
	cols, err := ParseExportRows("source,path,tokenName")
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 3 {
		t.Fatalf("expected 3 columns, got %v", cols)
	}

	_, err = ParseExportRows("badColumn")
	if err == nil {
		t.Fatal("expected error for invalid column")
	}
}

func TestActivityExportHeader(t *testing.T) {
	header := ExportHeader(true, []string{"source", "tokenName"})
	want := []string{"id", "createdAt", "username", "eventType", "source", "tokenName", "ipAddress", "details"}
	if len(header) != len(want) {
		t.Fatalf("header len %d != %d: %v", len(header), len(want), header)
	}
	for i, w := range want {
		if header[i] != w {
			t.Fatalf("header[%d]=%q want %q", i, header[i], w)
		}
	}
}

func TestSanitizeCSVCell(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"alice", "alice"},
		{"=1+1", "'=1+1"},
		{"+1234", "'+1234"},
		{"-1234", "'-1234"},
		{"@SUM(A1)", "'@SUM(A1)"},
	}
	for _, tc := range tests {
		if got := SanitizeCSVCell(tc.in); got != tc.want {
			t.Fatalf("SanitizeCSVCell(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
