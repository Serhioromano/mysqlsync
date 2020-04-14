package msc

import (
	"encoding/json"
	"strings"
	"fmt"
	"os"
)

// Snash make DB model snapshot and saves to file
func Snash(p Config) error {
	path := p.FilesPath
	file := p.File

	fmt.Println(path)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		err = os.MkdirAll(path, 0755)
		if err != nil {
			return err
		}
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
		p.User,
		p.Pass,
		p.Host,
		p.Port,
		p.DB,
	)

	db := DBConn{}
	err = db.SQLConnect(dsn)
	if err != nil {
		return err
	}
	fmt.Printf("Connected on: %s \n", dsn)

	f, err := os.Create(path + "/" + file)
	defer f.Close()
	if err != nil {
		return err
	}

	db.Prefix = p.Prefix
	db.Scheme = p.DB

	tables, err := db.GetTables()
	if err != nil {
		return err
	}

	var save = make(map[string]interface{})

	save["name"] = db.Scheme
	save["prefix"] = db.Prefix

	allTables := make(map[string]interface{})
	for _, table := range tables {
		saveTable := Struct2json(table)

		// Get fields
		fields, err := db.GetFields(table.Name)
		if err != nil {
			panic(err.Error())
		}
		fieldsMap := make(map[string]interface{})
		for _, field := range fields {
			saveField := Struct2json(field)
			fieldsMap[field.ORDINAL_POSITION] = saveField
		}
		saveTable["fields"] = fieldsMap

		// Get indexes
		indexes, err := db.GetIndexes(table.Name)
		if err != nil {
			panic(err.Error())
		}
		indexMap := make(map[string]interface{})
		flds := make(map[string][]string)
		for _, index := range indexes {
			saveIndex := Struct2json(index)
			flds[index.Key_name] = append(flds[index.Key_name], index.Column_name)
			saveIndex["fields"] = flds[index.Key_name] 
			indexMap[index.Key_name] = saveIndex
			
			if index.Key_name == "PRIMARY" {
				saveTable["Primary"] = index.Column_name
			}
		}
		saveTable["indexes"] = indexMap

		// Get constrain
		constraines, err := db.GetConstraines(table.Name)
		if err != nil {
			panic(err.Error())
		}
		constrainMap := make(map[string]interface{})
		for _, constrain := range constraines {
			saveconstrain := Struct2json(constrain)
			constrainMap[constrain.CONSTRAINT_NAME] = saveconstrain
		}

		saveTable["constraines"] = constrainMap
		saveTableName := strings.Replace(table.Name, p.Prefix, "", 1)
		saveTable["Name"] = saveTableName

		allTables[saveTableName] = saveTable
	}

	save["tables"] = allTables
	
	out, err := json.Marshal(save)
	fmt.Println(err)
	f.WriteString(string(out))

	return nil
}
