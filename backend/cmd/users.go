package cmd

import (
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/gtsteffaniak/filebrowser/users"
)

func init() {
	rootCmd.AddCommand(usersCmd)
}

var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "Users management utility",
	Long:  `Users management utility.`,
	Args:  cobra.NoArgs,
}

func printUsers(usrs []*users.User) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0) //nolint:gomnd
	fmt.Fprintln(w, "ID\tUsername\tScope\tLocale\tV. Mode\tS.Click\tAdmin\tExecute\tCreate\tRename\tModify\tDelete\tShare\tDownload\tPwd Lock")

	for _, u := range usrs {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%t\t%t\t%t\t%t\t%t\t%t\t%t\t%t\t%t\t%t\t\n",
			u.ID,
			u.Username,
			u.Scope,
			u.Locale,
			u.ViewMode,
			u.SingleClick,
			u.Perm.Admin,
			u.Perm.Execute,
			u.Perm.Create,
			u.Perm.Rename,
			u.Perm.Modify,
			u.Perm.Delete,
			u.Perm.Share,
			u.Perm.Download,
			u.LockPassword,
		)
	}

	w.Flush()
}

func parseUsernameOrID(arg string) (username string, id uint) {
	id64, err := strconv.ParseUint(arg, 10, 64)
	if err != nil {
		return arg, 0
	}
	return "", uint(id64)
}
