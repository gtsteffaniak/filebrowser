package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func init() {
	configCmd.AddCommand(configSetCmd)
	addConfigFlags(configSetCmd.Flags())
}

var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Updates the configuration",
	Long: `Updates the configuration. Set the flags for the options
you want to change. Other options will remain unchanged.`,
	Args: cobra.NoArgs,
	Run: python(func(cmd *cobra.Command, args []string, d pythonData) {
		flags := cmd.Flags()
		set, err := d.store.Settings.Get()
		checkErr(err)

		ser, err := d.store.Settings.GetServer()
		checkErr(err)

		flags.Visit(func(flag *pflag.Flag) {
			switch flag.Name {
			case "baseurl":
				ser.BaseURL = mustGetString(flags, flag.Name)
			case "root":
				ser.Root = mustGetString(flags, flag.Name)
			case "socket":
				ser.Socket = mustGetString(flags, flag.Name)
			case "cert":
				ser.TLSCert = mustGetString(flags, flag.Name)
			case "key":
				ser.TLSKey = mustGetString(flags, flag.Name)
			case "address":
				ser.Address = mustGetString(flags, flag.Name)
			case "port":
				ser.Port = 8080
			case "log":
				ser.Log = mustGetString(flags, flag.Name)
			case "signup":
				set.Signup = mustGetBool(flags, flag.Name)
			case "shell":
				set.Shell = convertCmdStrToCmdArray(mustGetString(flags, flag.Name))
			case "frontend.name":
				set.Frontend.Name = mustGetString(flags, flag.Name)
			case "frontend.color":
				set.Frontend.Color = mustGetString(flags, flag.Name)
			case "frontend.disableExternal":
				set.Frontend.DisableExternal = mustGetBool(flags, flag.Name)
			case "frontend.disableUsedPercentage":
				set.Frontend.DisableUsedPercentage = mustGetBool(flags, flag.Name)
			case "frontend.files":
				set.Frontend.Files = mustGetString(flags, flag.Name)
			}
		})

		getUserDefaults(flags, &set.UserDefaults, false)

		// read the defaults
		auther := getAuthentication()
		err = d.store.Auth.Save(auther)
		checkErr(err)
		err = d.store.Settings.Save(set)
		checkErr(err)
		err = d.store.Settings.SaveServer(ser)
		checkErr(err)
		printSettings(ser, set, auther)
	}, pythonConfig{}),
}
