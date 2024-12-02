package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/gtsteffaniak/filebrowser/backend/storage"
	"github.com/gtsteffaniak/filebrowser/backend/users"
	"github.com/gtsteffaniak/filebrowser/backend/utils"
)

func init() {
	usersCmd.AddCommand(usersImportCmd)
	usersImportCmd.Flags().Bool("overwrite", false, "overwrite users with the same id/username combo")
	usersImportCmd.Flags().Bool("replace", false, "replace the entire user base")
}

var usersImportCmd = &cobra.Command{
	Use:   "import <path>",
	Short: "Import users from a file",
	Long: `Import users from a file. The path must be for a json or yaml
file. You can use this command to import new users to your
installation. For that, just don't place their ID on the files
list or set it to 0.`,
	Args: jsonYamlArg,
	Run: cobraCmd(func(cmd *cobra.Command, args []string, store *storage.Storage) {
		fd, err := os.Open(args[0])
		utils.CheckErr("os.Open", err)
		defer fd.Close()

		list := []*users.User{}
		err = unmarshal(args[0], &list)
		utils.CheckErr("unmarshal", err)

		if mustGetBool(cmd.Flags(), "replace") {
			oldUsers, err := store.Users.Gets("")
			utils.CheckErr("store.Users.Gets", err)

			err = marshal("users.backup.json", list)
			utils.CheckErr("marshal users.backup.json", err)

			for _, user := range oldUsers {
				err = store.Users.Delete(user.ID)
				utils.CheckErr("store.Users.Delete", err)
			}
		}

		overwrite := mustGetBool(cmd.Flags(), "overwrite")

		for _, user := range list {
			onDB, err := store.Users.Get("", user.ID)

			// User exists in DB.
			if err == nil {
				if !overwrite {
					newErr := errors.New("user " + strconv.Itoa(int(user.ID)) + " is already registered")
					utils.CheckErr("", newErr)
				}

				// If the usernames mismatch, check if there is another one in the DB
				// with the new username. If there is, print an error and cancel the
				// operation
				if user.Username != onDB.Username {
					if conflictuous, err := store.Users.Get("", user.Username); err == nil { //nolint:govet
						newErr := usernameConflictError(user.Username, conflictuous.ID, user.ID)
						utils.CheckErr("usernameConflictError", newErr)
					}
				}
			} else {
				// If it doesn't exist, set the ID to 0 to automatically get a new
				// one that make sense in this DB.
				user.ID = 0
			}

			err = store.Users.Save(user)
			utils.CheckErr("store.Users.Save", err)
		}
	}),
}

func usernameConflictError(username string, originalID, newID uint) error {
	return fmt.Errorf(`can't import user with ID %d and username "%s" because the username is already registred with the user %d`,
		newID, username, originalID)
}
