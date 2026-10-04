package web

import (
	"os"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"golang.org/x/crypto/bcrypt"
)

func TestMain(m *testing.M) {
	utils.BcryptCost = bcrypt.MinCost
	if err := utils.SetInvalidPasswordHash(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}
