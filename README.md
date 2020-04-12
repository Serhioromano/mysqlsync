# MySQL Migration tool on steroids

As we all know migrations is pain. It has so many points of failer and those who once was in that cycle, knows how hard it is to get out.

> Disclaimer: Works only with MySQL DB

`mysqlsync` is a completely new approach to DB migrations. Instead of creating modification steps, you make a snapshot (`snash`) of a DB structure to JSON file. And then you restore this file to another DB. It will compare restore DB against JSON and automatically generate migration queries to alter columns, indexes, create or delete tables and constraints.

You can use it as CLI tool as programmatically.

## Install

```
go get github.com/serhioromano/mysqlsync
```

todo:
- Install with NPM
- Install with yum
- Install with apt

## Use CLI

Now you can use it as CLI. To create snapshot you run command

```
mysqlsync snash [options]
```

You can see documentation for all options using `mysqlsync snash --help` command. If you do not want to pass all parameters becase there are a lot of them, you can create profile file `.mysqlsync.json`. Here is an example

```json
{
    "files_path" : "./snash",
    "profiles": {
        "dev":{
            "dbname": "icod_project",
            "user": "root",
            "pass": "root",
            "host": "localhost",
            "port": "3306",
            "prefix": "",
        },
        "prod":{
            "dbname": "p_8",
            "user": "root",
            "pass": "root",
            "host": "localhost",
            "port": "3306",
            "prefix": "p_8_",
            "file_name": "icod_project.json",
            "delete_table": true,
            "delete_column": true,
            "delete_index": true
            "delete_constraint": true
        }
    }
}
```

Now you can call CLI tool with only one parameter `-p` or `--profile`

```
mysqlsync snash -p=dev
mysqlsync restore -p=prod
```

First command create file `./snash/icod_project.json` with `icod_project` DB model snapshot and second command restore it to `p_8` DB with new prefix.

## Use programmatically

### Snash

```go
package main

import (
	"fmt"
	"github.com/serhioromano/mysqlsync/msc"
)

func main() {
	options := msc.MSCConfig{
		User:        "root",
		Pass:        "root",
		Host:        "localhost",
		Port:        "3306",
		DB:          "test",
		FilesPath:   "./snash",
		Prefix:      "prefix",
		File:        "test.json",
	}
	err := msc.Snash(options)
	if err != nil {
		panic(err.Error())
	}
	fmt.Println("Snapshot done")
}
```

### Restore

```go
package main

import (
	"fmt"
	"github.com/serhioromano/mysqlsync/msc"
)

func main() {
	options := msc.MSCConfig{
		User:        "root",
		Pass:        "root",
		Host:        "localhost",
		Port:        "3306",
		DB:          "test",
		FilesPath:   "./snash",
		Prefix:      "prefix",
		File:        "test.json",
		DTable:      true,
		DColumn:     true,
		DIndex:      true,
		DConstraint: true,
	}
	runner := msc.Restore{}
	err := runner.Run(options)
	if err != nil {
		panic(err.Error())
	}
	fmt.Println("Snapshot restored")
}
```