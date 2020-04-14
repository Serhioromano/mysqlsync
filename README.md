# MySQL Migration tool on steroids

As we all know migrations is pain. It has so many points of failer and those who once was in that cycle, knows how hard it is to get out.

> **Disclaimer:** 
> 1. Works only with MySQL DB
> 2. All foreign keys (constraints) HAVE TO have `fk_` prefix
> 3. All primary fields autoincremented preferably named `id` (not necessary)

`mysqlsync` is a completely new approach to DB migrations. Instead of creating modification steps, you make a snapshot (`snash`) of a DB structure to JSON file. And then you restore this file to another DB. It will compare restore DB against JSON and automatically generate migration queries to alter columns, indexes, create or delete tables and constraints.

## What it supports

1. Creation deletion of tables
2. Altering and deleting table fields
3. Altering and deleting indexes
4. Altering constraints
5. [planned] updating views
6. [planned] updating routines
7. [planned] updating triggers

You can use it as CLI tool as programmatically.

## Install

```
go get -u github.com/serhioromano/mysqlsync
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

You can see documentation for all options using `mysqlsync snash --help` command. If you do not want to pass all parameters because there are a lot of them, you can create profile file `.mysqlsync.json`. Here is an example

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
            "delete_index": true,
            "delete_constraint": true,
            "optimise": true
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

### Parameters

#### Global for all commands

JSON Param | Flag | Description
---|---|---
-- | -h, --help | help for command
-- | --config | Config file to load (default is $PWD/.mysqlsync.json)
-- | -p, --profile | Name of connection profile in configuration file. By adding profile you do not need to add any other flag.
`path` | --path | Path where snapshots are stored
`dbname` | --db | DB scheme name
`file` | -f, --file | File to save snapshot or to restore from (default to DB scheme name)
`host` | --host | DB host
`pass` | --pass | DB password
`port` | --port | DB port
`prefix` | --prefix | DB table prefix. Will be deleted on snapshot and added on restore, thus you can have dev tables with one prefix or without and prod tables with other prefix.
`user` | --user | DB user name

#### For restore command

JSON Param | Flag | Description
---|---|---
`delete_column` | -c, --d-column | Delete table columns that are not in the import (default true)
`delete_constraint` | -k, --d-constraint | Delete constraints that are not in the import (default true)
`delete_index` | -i, --d-index | Delete indexes that are not in the import (default true)
`delete_table` | -t, --d-table | Delete tables that are not in the import (default true)
`optimise`| -o, --optimize | Run optimize query on table after finish (default true). It only run for MyIsam and InnoDB table types.

## Use programmatically

### Snash

```go
package main

import (
	"fmt"
	"github.com/serhioromano/mysqlsync/msc"
)

func main() {
	options := msc.Config{
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
	options := msc.Config{
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
		Optimize: true,
	}
	runner := msc.Restore{}
	err := runner.Run(options)
	if err != nil {
		panic(err.Error())
	}
	fmt.Println("Snapshot restored")
}
```
