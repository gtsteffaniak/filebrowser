// Package testutil holds helpers shared by test binaries. It must not be
// imported by production code.
package testutil

import (
	"log"
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/go-logger/logger"
)

// Main lowers the bcrypt cost before running the package's tests. Under the
// race detector a single bcrypt hash at the default cost takes most of a
// second, which dominates any test that creates a user (state.Initialize runs
// QuickSetup, so every fresh database pays it). Call it from TestMain:
//
//	func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }
//
// It also quiets INFO/DEBUG/WARN logger output so failures are readable.
// Set FILEBROWSER_TEST_LOG=1 to keep normal logging.
func Main(m *testing.M) int {
	utils.BcryptCost = bcrypt.MinCost
	QuietLogging()
	return m.Run()
}

// QuietLogging sends only error/fatal lines to the null device. Safe to call
// multiple times; no-ops when FILEBROWSER_TEST_LOG is set.
func QuietLogging() {
	if os.Getenv("FILEBROWSER_TEST_LOG") != "" {
		return
	}
	logger.DisableCompatibilityMode()
	if err := logger.EnableCompatibilityMode(logger.JsonConfig{
		Levels:   "error|fatal",
		Output:   os.Stderr.Name(),
		NoColors: true,
	}); err != nil {
		log.Println("[ERROR] Failed to set up quiet test logger:", err)
	}
}
