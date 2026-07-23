package files

import (
	"encoding/json"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
)

func TestItemsJSON_omitsEmptySlices(t *testing.T) {
	items := Items{
		Files:   utils.NonNilSlice([]string(nil)),
		Folders: utils.NonNilSlice([]string(nil)),
	}
	raw, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("marshal items: %v", err)
	}
	if string(raw) != "{}" {
		t.Fatalf("JSON = %s, want {}", string(raw))
	}
}

func TestItemsJSON_omitsEmptyFilesWhenFoldersPresent(t *testing.T) {
	items := Items{
		Folders: []string{"docs"},
	}
	raw, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("marshal items: %v", err)
	}
	const want = `{"folders":["docs"]}`
	if string(raw) != want {
		t.Fatalf("JSON = %s, want %s", string(raw), want)
	}
}

func TestItemsJSON_marshalsPopulatedSlices(t *testing.T) {
	items := Items{
		Files:   []string{"a.txt"},
		Folders: []string{"docs"},
	}
	raw, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("marshal items: %v", err)
	}
	const want = `{"files":["a.txt"],"folders":["docs"]}`
	if string(raw) != want {
		t.Fatalf("JSON = %s, want %s", string(raw), want)
	}
}
