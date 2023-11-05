package files

import (
	"sort"
	"strings"

	"github.com/maruel/natural"
)

// Sorting constants
const (
	SortingByName     = "name"
	SortingBySize     = "size"
	SortingByModified = "modified"
)

// Listing is a collection of files.
type Listing struct {
	Items    []*FileInfo `json:"items"`
	NumDirs  int         `json:"numDirs"`
	NumFiles int         `json:"numFiles"`
	Sorting  Sorting     `json:"sorting"`
}

// SortingSettings represents the sorting settings.
type Sorting struct {
	By  string `json:"by"`
	Asc bool   `json:"asc"`
}

// ApplySort applies the specified sorting order to the listing.
func (l *Listing) ApplySort() {
	less := func(i, j int) bool {
		switch l.Sorting.By {
		case SortingByName:
			return natural.Less(strings.ToLower(l.Items[i].Name), strings.ToLower(l.Items[j].Name))
		case SortingBySize:
			return l.sortBySize(i, j)
		case SortingByModified:
			return l.sortByModified(i, j)
		default:
			return false
		}
	}
	if l.Sorting.Asc == false {
		sortItems(l.Items, func(i, j int) bool {
			return !less(i, j)
		})
	} else {
		sortItems(l.Items, less)
	}
}

// sortItems is a generic sorting function for items.
func sortItems(items []*FileInfo, less func(i, j int) bool) {
	sort.Slice(items, func(i, j int) bool {
		return less(i, j)
	})
}

const directoryOffset = -1 << 31 // = math.MinInt32

// sortBySize sorts items by size.
func (l *Listing) sortBySize(i, j int) bool {
	iSize, jSize := l.Items[i].Size, l.Items[j].Size
	if l.Items[i].IsDir {
		iSize = directoryOffset + iSize
	}
	if l.Items[j].IsDir {
		jSize = directoryOffset + jSize
	}
	return iSize < jSize
}

// sortByModified sorts items by modification time.
func (l *Listing) sortByModified(i, j int) bool {
	iModified, jModified := l.Items[i].ModTime, l.Items[j].ModTime
	return iModified.Sub(jModified) < 0
}
