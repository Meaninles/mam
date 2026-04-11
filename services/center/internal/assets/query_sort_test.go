package assets

import (
	"testing"

	assetdto "mare/shared/contracts/dto/asset"
)

func TestSortEntriesForBrowseKeepsFolderOrderWhenSortingByRating(t *testing.T) {
	items := []assetdto.EntryRecord{
		{ID: "dir-b", Type: "folder", Name: "B目录", Rating: 5, ModifiedAt: "2026-04-10 10:00"},
		{ID: "dir-a", Type: "folder", Name: "A目录", Rating: 1, ModifiedAt: "2026-04-10 09:00"},
		{ID: "file-low", Type: "file", Name: "low.jpg", Rating: 1, ModifiedAt: "2026-04-10 09:00"},
		{ID: "file-high", Type: "file", Name: "high.jpg", Rating: 5, ModifiedAt: "2026-04-10 10:00"},
	}

	sortEntriesForBrowse(items, "星级", "desc")

	got := []string{items[0].ID, items[1].ID, items[2].ID, items[3].ID}
	want := []string{"dir-b", "dir-a", "file-high", "file-low"}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("unexpected order: got=%v want=%v", got, want)
		}
	}
}
