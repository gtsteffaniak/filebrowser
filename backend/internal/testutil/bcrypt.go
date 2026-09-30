// Package testutil holds helpers shared by test binaries. It must not be
// imported by production code.
package testutil

import (
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
)

// Main lowers the bcrypt cost before running the package's tests. Under the
// race detector a single bcrypt hash at the default cost takes most of a
// second, which dominates any test that creates a user (state.Initialize runs
// QuickSetup, so every fresh database pays it). Call it from TestMain:
//
//	func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }
func Main(m *testing.M) int {
	utils.BcryptCost = bcrypt.MinCost
	return m.Run()
}
