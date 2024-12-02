package cmd

import (
	"log"

	"github.com/gtsteffaniak/filebrowser/backend/storage"
	"github.com/gtsteffaniak/filebrowser/backend/utils"
	"github.com/spf13/cobra"
)

func init() {
	usersCmd.AddCommand(usersRmCmd)
}

var usersRmCmd = &cobra.Command{
	Use:   "rm <id|username>",
	Short: "Delete a user by username or id",
	Long:  `Delete a user by username or id`,
	Args:  cobra.ExactArgs(1),
	Run: cobraCmd(func(cmd *cobra.Command, args []string, store *storage.Storage) {
		username, id := parseUsernameOrID(args[0])
		var err error

		if username != "" {
			err = store.Users.Delete(username)
		} else {
			err = store.Users.Delete(id)
		}

		utils.CheckErr("usersRmCmd", err)
		log.Println("user deleted successfully")
	}),
}
