package sqldb

import (
	"os"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/testutil"
)

func TestMain(m *testing.M) {
	os.Exit(testutil.Main(m))
}
