package cmd

import (
	"github.com/gtsteffaniak/filebrowser/storage"
	"github.com/spf13/cobra"
)

func init() {
	usersCmd.AddCommand(usersExportCmd)
}

var usersExportCmd = &cobra.Command{
	Use:   "export <path>",
	Short: "Export all users to a file.",
	Long: `Export all users to a json or yaml file. Please indicate the
path to the file where you want to write the users.`,
	Args: jsonYamlArg,
	Run: python(func(cmd *cobra.Command, args []string, store *storage.Storage) {
		list, err := d.store.Users.Gets("")
		checkErr("d.store.Users.Gets", err)

		err = marshal(args[0], list)
		checkErr("marshal", err)
	}, pythonConfig{}),
}
