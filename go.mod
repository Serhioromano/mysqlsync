module github.com/serhioromano/mysqlsync

go 1.16

require (
	github.com/fatih/color v1.10.0
	github.com/go-sql-driver/mysql v1.5.0
	github.com/spf13/cobra v1.1.3
	github.com/spf13/viper v1.7.1
)

replace github.com/serhioromano/mysqlsync/cmd => ../cmd
