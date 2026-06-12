// Package cmd CLI tool package
// Copyright © 2020 Sergey Romanov aka Serhioromano <serg4172@mail.ru>
package cmd

import (
	"fmt"

	"github.com/serhioromano/mysqlsync/msc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// snashCmd represents the snash command
var snashCmd = &cobra.Command{
	Use:   "snash",
	Short: "Create DB model snapshot",
	Long: `Creates DB model snapshot and stores it as a DBML file (Database Markup Language).
You can use this file to conform another DB to this model with the restore command.
DBML files can be visualized at https://dbdiagram.io/home`,
	Run: func(cmd *cobra.Command, args []string) {
		options := msc.Config{
			User:      viper.GetString("user"),
			Pass:      viper.GetString("pass"),
			Host:      viper.GetString("host"),
			Port:      viper.GetString("port"),
			DB:        viper.GetString("db"),
			FilesPath: viper.GetString("path"),
			File:      viper.GetString("file"),
			Prefix:    viper.GetString("prefix"),
		}
		err := msc.Snash(options)
		if err != nil {
			panic(err.Error())
		}
		fmt.Println("Snapshot created successfuly: " + viper.GetString("path") + "/" + viper.GetString("file"))
	},
}

func init() {
	rootCmd.AddCommand(snashCmd)
}
