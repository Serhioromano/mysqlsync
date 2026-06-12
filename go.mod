module github.com/serhioromano/mysqlsync

go 1.16

require (
	github.com/go-sql-driver/mysql v1.5.0
	github.com/mattn/go-sqlite3 v1.14.22
	github.com/spf13/cobra v1.1.3
	github.com/spf13/viper v1.7.1
	golang.org/x/sys v0.0.0-20200223170610-d5e6a3e2c0ae // indirect
)

replace github.com/serhioromano/mysqlsync/cmd => ../cmd
