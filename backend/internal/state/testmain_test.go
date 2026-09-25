package state

import (
	"os"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"golang.org/x/crypto/bcrypt"
)

func TestMain(m *testing.M) {
	utils.BcryptCost = bcrypt.MinCost
	os.Exit(m.Run())
}
