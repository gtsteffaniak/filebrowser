package indexing

import (
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"golang.org/x/net/webdav"
)

// SetTestIndex allows tests to register mock indices without database initialization
// This is useful for testing code that depends on GetIndex() without needing full setup
func SetTestIndex(name string, path string) {
	indexesMutex.Lock()
	defer indexesMutex.Unlock()

	idx := &Index{
		Source: settings.Source{
			Name: name,
			Path: path,
		},
		WebdavLock: webdav.NewMemLS(),
		mock:       true,
	}
	indexes[name] = idx
}

// ClearTestIndices removes all test indices - call in test cleanup
func ClearTestIndices() {
	indexesMutex.Lock()
	defer indexesMutex.Unlock()
	indexes = make(map[string]*Index)
}
