// Package cmd CLI tool package
// Copyright © 2020 Sergey Romanov aka Serhioromano <serg4172@mail.ru>
package cmd

import (
	"fmt"

	"github.com/serhioromano/mysqlsync/msc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// restoreCmd represents the restore command
var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore DB model from shapshot",
	Long:  `It takes .json file with DB model snapshot created by shash command and run conform DB to that snapshot`,
	Run: func(cmd *cobra.Command, args []string) {

		profile := viper.GetString("profile")
		if profile != "" {
			if viper.IsSet("profiles." + profile + ".delete_table") {
				viper.Set("delete_table", viper.Get("profiles."+profile+".delete_table"))
			}
			if viper.IsSet("profiles." + profile + ".delete_index") {
				viper.Set("delete_index", viper.Get("profiles."+profile+".delete_index"))
			}
			if viper.IsSet("profiles." + profile + ".delete_column") {
				viper.Set("delete_column", viper.Get("profiles."+profile+".delete_column"))
			}
			if viper.IsSet("profiles." + profile + ".delete_constraint") {
				viper.Set("delete_constraint", viper.Get("profiles."+profile+".delete_constraint"))
			}
		}
		if t, _ := cmd.Flags().GetBool("d-table"); t == true {
			viper.Set("delete_table", t)
		}
		if c, _ := cmd.Flags().GetBool("d-column"); c == true {
			viper.Set("delete_column", c)
		}
		if i, _ := cmd.Flags().GetBool("d-index"); i == true {
			viper.Set("delete_index", i)
		}
		if k, _ := cmd.Flags().GetBool("d-constraint"); k == true {
			viper.Set("delete_constraint", k)
		}

		options := msc.MSCConfig{
			User:        viper.GetString("user"),
			Pass:        viper.GetString("pass"),
			Host:        viper.GetString("host"),
			Port:        viper.GetString("port"),
			DB:          viper.GetString("db"),
			FilesPath:   viper.GetString("files_path"),
			File:        viper.GetString("file"),
			Prefix:      viper.GetString("prefix"),
			DTable:      viper.GetBool("delete_table"),
			DColumn:     viper.GetBool("delete_column"),
			DIndex:      viper.GetBool("delete_index"),
			DConstraint: viper.GetBool("delete_constraint"),
		}
		fmt.Println(options)
		runner := msc.Restore{}
		err := runner.Run(options)
		if err != nil {
			panic(err.Error())
		}
		fmt.Println("Snapshot restored: " + viper.GetString("files_path") + "/" + viper.GetString("file"))
	},
}

func init() {
	rootCmd.AddCommand(restoreCmd)

	restoreCmd.Flags().BoolP("d-table", "t", true, "Delete tables that are not in the import")
	restoreCmd.Flags().BoolP("d-index", "i", true, "Delete indexes that are not in the import")
	restoreCmd.Flags().BoolP("d-column", "c", true, "Delete table columns that are not in the import")
	restoreCmd.Flags().BoolP("d-constraint", "k", true, "Delete constraints that are not in the import")
}
