package cmd

import (
	"regexp"

	"github.com/spf13/cobra"

	"github.com/gtsteffaniak/filebrowser/rules"
	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/storage"
	"github.com/gtsteffaniak/filebrowser/users"
	"github.com/gtsteffaniak/filebrowser/utils"
)

func init() {
	rulesCmd.AddCommand(rulesAddCmd)
	rulesAddCmd.Flags().BoolP("allow", "a", false, "indicates this is an allow rule")
	rulesAddCmd.Flags().BoolP("regex", "r", false, "indicates this is a regex rule")
}

var rulesAddCmd = &cobra.Command{
	Use:   "add <path|expression>",
	Short: "Add a global rule or user rule",
	Long:  `Add a global rule or user rule.`,
	Args:  cobra.ExactArgs(1),
	Run: cobraCmd(func(cmd *cobra.Command, args []string, store *storage.Storage) {
		allow := mustGetBool(cmd.Flags(), "allow")
		regex := mustGetBool(cmd.Flags(), "regex")
		exp := args[0]

		if regex {
			regexp.MustCompile(exp)
		}

		rule := rules.Rule{
			Allow: allow,
			Regex: regex,
		}

		if regex {
			rule.Regexp = &rules.Regexp{Raw: exp}
		} else {
			rule.Path = exp
		}

		user := func(u *users.User) {
			u.Rules = append(u.Rules, rule)
			err := store.Users.Save(u)
			utils.CheckErr("store.Users.Save", err)
		}

		global := func(s *settings.Settings) {
			s.Rules = append(s.Rules, rule)
			err := store.Settings.Save(s)
			utils.CheckErr("store.Settings.Save", err)
		}

		runRules(store, cmd, user, global)
	}),
}
