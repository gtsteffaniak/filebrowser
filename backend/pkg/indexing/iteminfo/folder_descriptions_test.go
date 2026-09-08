package iteminfo

import "testing"

func TestFolderDescriptionsAreExactAndLimitedToVisibleFolders(t *testing.T) {
	info := FileInfo{Path: "/jobs/", Folders: []ItemInfo{{Name: "out"}, {Name: "other"}}, Files: []ExtendedItemInfo{{ItemInfo: ItemInfo{Name: "file.txt"}}}}
	info.SetFolderDescriptions(map[string]string{"/jobs/out": "Queue <plain text>", "/out": "Wrong level", "/jobs/file.txt": "Not a folder", "/jobs/hidden": "Not visible"})
	if info.Folders[0].FolderDescription != "Queue <plain text>" || info.Folders[1].FolderDescription != "" || info.Files[0].FolderDescription != "" {
		t.Fatalf("unexpected descriptions: %+v", info)
	}
	if len(info.Folders) != 2 {
		t.Fatal("annotation created a hidden folder")
	}
	info.SetFolderDescriptions(nil)
	if info.Folders[0].FolderDescription != "" {
		t.Fatal("removed configuration left a stale description")
	}
}
