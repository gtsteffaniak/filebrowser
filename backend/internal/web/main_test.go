package web

import (
	"os"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/testutil"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
)

func TestMain(m *testing.M) {
	if err := utils.SetInvalidPasswordHash(); err != nil {
		panic(err)
	}
	os.Exit(testutil.Main(m))
}
