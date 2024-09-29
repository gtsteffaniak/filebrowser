package cmd

import (
	"github.com/gtsteffaniak/filebrowser/storage"
	"github.com/spf13/cobra"
)

func init() {
	rulesCmd.AddCommand(rulesLsCommand)
}

var rulesLsCommand = &cobra.Command{
	Use:   "ls",
	Short: "List global rules or user specific rules",
	Long:  `List global rules or user specific rules.`,
	Args:  cobra.NoArgs,
	Run: cobraCmd(func(cmd *cobra.Command, args []string, store *storage.Storage) {
		runRules(store, cmd, nil, nil)
	}),
}
