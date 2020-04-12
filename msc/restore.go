package msc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/fatih/color"
)

// Restore DB model from file
type Restore struct {
	prefix string
	tables *[]Table
	conn   DBConn
}

// Run DB model from file
func (r *Restore) Run(p MSCConfig) error {
	path := p.FilesPath
	file := p.File

	dat, err := ioutil.ReadFile(path + "/" + file)
	if err != nil {
		return err
	}
	data := make(map[string]interface{})
	err = json.Unmarshal(dat, &data)
	if err != nil {
		return err
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
	r.conn = db
	fmt.Printf("Connected on: %s \n", dsn)

	db.Prefix = p.Prefix
	r.prefix = p.Prefix
	db.Scheme = p.DB

	r.runSQL("USE " + db.Scheme)
	r.runSQL("SET @@session.sql_mode =\"ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER,NO_ENGINE_SUBSTITUTION\"")

	currentTables, err := db.GetTables()
	if err != nil {
		return err
	}
	importTables := data["tables"]
	for _, importTable := range importTables.(map[string]interface{}) {
		newTableName := db.Prefix + getString(importTable, "Name")

		if currentTable, ok := importTableInCurrent(currentTables, newTableName); ok == true {
			// UPdate table
			if currentTable.Collation != getString(importTable, "Collation") ||
				currentTable.Engine != getString(importTable, "Engine") ||
				currentTable.Comment != getString(importTable, "Comment") {
				r.runSQL(fmt.Sprintf(
					`ALTER TABLE %s COLLATE = %s , ENGINE = %s, COMMENT = '%s'`,
					newTableName, getString(importTable, "Collation"),
					getString(importTable, "Engine"),
					getString(importTable, "Comment")))
			}

			currentFields, err := db.GetFields(getString(importTable, "Name"))
			if err != nil {
				return err
			}
			importFields := importTable.(map[string]interface{})["fields"]
			for _, importField := range importFields.(map[string]interface{}) {
				if getString(importField, "COLUMN_NAME") == getString(importTable, "Primary") {
					continue
				}

				nulable := "NULL"
				if getString(importField, "IS_NULLABLE") == "NO" {
					nulable = "NOT NULL"
				}

				ufs := fmt.Sprintf("`%s` %s %s %s COMMENT '%s'",
					getString(importField, "COLUMN_NAME"),
					getString(importField, "COLUMN_TYPE"),
					nulable,
					getDefault(importField),
					getString(importField, "COLUMN_COMMENT"),
				)

				if currField, ok := importFieldInCurrent(currentFields, getString(importField, "COLUMN_NAME")); ok == true {
					// Update field
					if currField.IS_NULLABLE != getString(importField, "IS_NULLABLE") ||
						currField.COLUMN_TYPE != getString(importField, "COLUMN_TYPE") ||
						(currField.COLUMN_DEFAULT != nil && *currField.COLUMN_DEFAULT != getString(importField, "COLUMN_DEFAULT")) ||
						currField.COLUMN_COMMENT != getString(importField, "COLUMN_COMMENT") {
						r.runSQL(fmt.Sprintf(
							`ALTER TABLE `+"`%s`"+` CHANGE COLUMN `+"`%s`"+` %s`,
							newTableName, getString(importField, "COLUMN_NAME"), ufs))
					}
				} else {
					// Create field
					r.runSQL(fmt.Sprintf(`ALTER TABLE `+"`%s`"+` ADD COLUMN %s`,
						newTableName, ufs))
				}

			}

			if p.DColumn {
				for _, currentField := range currentFields {
					if !currentFieldInImport(importFields, currentField.COLUMN_NAME) {
						r.runSQL(fmt.Sprintf(`ALTER TABLE `+"`%s`"+` DROP COLUMN  `+"`%s`",
							newTableName, currentField.COLUMN_NAME))
					}
				}
			}

			currentIndexes, err := db.GetIndexes(getString(importTable, "Name"))
			if err != nil {
				return err
			}
			importIndexes := importTable.(map[string]interface{})["indexes"]
			for _, importIndex := range importIndexes.(map[string]interface{}) {
				if _, ok := importIndexInCurrent(currentIndexes, getString(importIndex, "Key_name")); ok == true {
					continue
				}
				if getString(importIndex, "Key_name") == "PRIMARY" {
					continue
				}
				if strings.Contains(getString(importIndex, "Key_name"), "fk_") {
					continue
				}

				r.runSQL(fmt.Sprintf("ALTER TABLE `%s` ADD INDEX `%s` (%s ASC)",
					newTableName,
					getString(importIndex, "Key_name"),
					joinI(importIndex.(map[string]interface{})["fields"], " ASC, ")))

			}

			if p.DIndex {
				for _, currentIndex := range currentIndexes {
					// If index is not in the import and it is not index of foreign key
					// delete it
					if !currentIndexInImport(importIndexes, currentIndex.Key_name) &&
						!strings.Contains(currentIndex.Key_name, "fk_") {
						r.runSQL(fmt.Sprintf("DROP INDEX `%s` ON %s",
							currentIndex.Key_name, newTableName))
					}
				}
			}

		} else {
			// Create table
			var sql []string

			constrains := importTable.(map[string]interface{})["constraines"].(map[string]interface{})
			indexes := importTable.(map[string]interface{})["indexes"].(map[string]interface{})
			fields := importTable.(map[string]interface{})["fields"].(map[string]interface{})

			for _, importField := range fields {

				if getString(importField, "COLUMN_NAME") == getString(importTable, "Primary") {
					sql = append(sql, fmt.Sprintf("`%s` %s NULL AUTO_INCREMENT", getString(importTable, "Primary"), getString(importField, "COLUMN_TYPE")))
					continue
				}
				nulable := "NULL"
				if getString(importField, "IS_NULLABLE") == "NO" {
					nulable = "NOT NULL"
				}
				sql = append(sql, fmt.Sprintf("`%s` %s %s %s COMMENT '%s'",
					getString(importField, "COLUMN_NAME"),
					getString(importField, "COLUMN_TYPE"),
					nulable,
					getDefault(importField),
					getString(importField, "COLUMN_COMMENT")))

			}
			for _, importIndex := range indexes {
				if isConstrain(getString(importIndex, "Key_name"), constrains) {
					continue
				}

				if getString(importIndex, "Key_name") == "PRIMARY" {
					sql = append(sql, fmt.Sprintf("PRIMARY KEY (%s)",
						joinI(importIndex.(map[string]interface{})["fields"], ", ")))
					continue
				}

				key := "KEY"
				if getString(importIndex, "Index_type") == "FULLTEXT" {
					key = "FULLTEXT KEY"
				}
				if getString(importIndex, "Non_unique") == "0" {
					key = "UNIQUE KEY"
				}

				sql = append(sql, fmt.Sprintf("%s `%s`(%s)",
					key,
					getString(importIndex, "Key_name"),
					joinI(importIndex.(map[string]interface{})["fields"], ", ")))

			}

			r.runSQL(fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s` (\n\t%s\n) ENGINE=%s COLLATE=%s COMMENT='%s'",
				newTableName,
				strings.Join(sql, ",\n\t"),
				getString(importTable, "Engine"),
				getString(importTable, "Collation"),
				getString(importTable, "Comment")))

		}
		// fmt.Println(res)
	}

	if p.DTable {
		for _, currentTable := range currentTables {
			if !currentTableInImport(p.Prefix, importTables, currentTable.Name) {
				r.runSQL(fmt.Sprintf("DROP TABLE IF EXISTS `%s`", currentTable.Name))
			}
		}
	}

	// Constrains
	r.runSQL("SET @@session.UNIQUE_CHECKS=0")
	r.runSQL("SET @@session.FOREIGN_KEY_CHECKS=0")
	r.runSQL("SET @@session.sql_mode='ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION'")
	currentTables, err = db.GetTables()
	if err != nil {
		return err
	}
	// r.runSQL("START TRANSACTION")
	for _, importTable := range data["tables"].(map[string]interface{}) {
		newTableName := db.Prefix + getString(importTable, "Name")
		currentConstrains, err := db.GetConstraines((getString(importTable, "Name")))
		if err != nil {
			return err
		}
		importConstrains := importTable.(map[string]interface{})["constraines"].(map[string]interface{})

		for _, currentConstrain := range currentConstrains {
			if p.DConstraint == false && !currentConstrainInImport(importConstrains, currentConstrain.CONSTRAINT_NAME, getString(data, "prefix"), p.Prefix) {
				continue
			}
			r.runSQL(fmt.Sprintf("ALTER TABLE `%s` DROP FOREIGN KEY `%s`",
				newTableName, currentConstrain.CONSTRAINT_NAME))
			// r.runSQL(fmt.Sprintf("ALTER TABLE `%s` DROP INDEX `%s`",
			// 	newTableName, currentConstrain.CONSTRAINT_NAME))
		}
		for _, importConstrain := range importConstrains {


			importTableName := p.Prefix + strings.Replace(getString(importConstrain, "REFERENCED_TABLE_NAME"), getString(data, "prefix"), "", 1)
			r.runSQL(fmt.Sprintf("ALTER TABLE `%s` ADD CONSTRAINT `%s` FOREIGN KEY (`%s`)"+
				" REFERENCES `%s` (`%s`) ON DELETE %s ON UPDATE %s",
				newTableName,
				p.Prefix+getString(importConstrain, "CONSTRAINT_NAME"),
				getString(importConstrain, "COLUMN_NAME"),
				importTableName,
				getString(importConstrain, "REFERENCED_COLUMN_NAME"),
				getString(importConstrain, "DELETE_RULE"),
				getString(importConstrain, "UPDATE_RULE")))
		}

		if getString(importTable, "Engine") == "InnoDB" || getString(importTable, "Engine") == "MyISAM" {
			r.runSQL("OPTIMIZE TABLE " + newTableName)
		}

	}
	// r.runSQL("SET SQL_MODE=@OLD_SQL_MODE")
	// r.runSQL("SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS")
	// r.runSQL("SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS")
	// r.runSQL("COMMIT")

	return nil
}

func (r *Restore) runSQL(sql string) error {
	color.Cyan(sql)
	fmt.Println("")
	_, err := r.conn.Conn.Query(sql)
	if err != nil {
		panic(err.Error())
	}
	return err
}

func joinI(i interface{}, glue string) string {
	list := i.([]interface{})
	var sslice []string

	for _, v := range list {
		if v.(string) == "" {
			continue
		}
		sslice = append(sslice, v.(string))
	}

	return strings.Join(sslice, glue)
}

func isConstrain(name string, c map[string]interface{}) bool {
	out := false

	for _, v := range c {
		if strings.Contains(name, getString(v, "CONSTRAINT_NAME")) {
			out = true
			break
		}
	}

	return out
}

func importIndexInCurrent(i []Index, name string) (Index, bool) {
	index := Index{}
	ok := false
	for _, t := range i {
		if t.Key_name == name {
			index = t
			ok = true
			break
		}
	}
	return index, ok
}
func currentIndexInImport(i interface{}, name string) bool {
	ok := false
	for _, f := range i.(map[string]interface{}) {
		if getString(f, "Key_name") == name {
			ok = true
			break
		}
	}
	return ok
}

func constrainExists(i []Constrain, cc map[string]interface{}, oldp string, newp string) bool {
	ok := false
	importTable := strings.Replace(getString(cc, "REFERENCED_TABLE_NAME"), oldp, "", 1)
	for _, c := range i {
		currentTable := strings.Replace(c.REFERENCED_COLUMN_NAME, newp, "", 1)
		if currentTable == importTable &&
			c.REFERENCED_COLUMN_NAME == getString(cc, "REFERENCED_COLUMN_NAME") &&
			c.CONSTRAINT_NAME == getString(cc, "CONSTRAINT_NAME") {
			ok = true
			break
		}
	}
	return ok
}
func currentConstrainInImport(i interface{}, name string, oldp string, newp string) bool {
	ok := false
	cname := strings.Replace(name, oldp, "", 1)
	for _, f := range i.(map[string]interface{}) {
		iname := strings.Replace(getString(f, "CONSTRAINT_NAME"), newp, "", 1)
		if cname == iname {
			ok = true
			break
		}
	}
	return ok
}
func importTableInCurrent(tables []Table, name string) (Table, bool) {
	table := Table{}
	ok := false
	for _, t := range tables {
		if t.Name == name {
			table = t
			ok = true
			break
		}
	}
	return table, ok
}
func currentTableInImport(p string, tables interface{}, name string) bool {
	ok := false
	for _, f := range tables.(map[string]interface{}) {
		if p+getString(f, "Name") == name {
			ok = true
			break
		}
	}
	return ok
}

func importFieldInCurrent(fields []Field, name string) (Field, bool) {
	field := Field{}
	ok := false
	for _, f := range fields {
		if f.COLUMN_NAME == name {
			field = f
			ok = true
			break
		}
	}
	return field, ok
}
func currentFieldInImport(fields interface{}, name string) bool {
	ok := false
	for _, f := range fields.(map[string]interface{}) {
		if getString(f, "COLUMN_NAME") == name {
			ok = true
			break
		}
	}
	return ok
}

func getDefault(field interface{}) string {
	def := getString(field, "COLUMN_DEFAULT")
	ftype := getString(field, "DATA_TYPE")

	if ftype == "int" && def == "" && getString(field, "IS_NULLABLE") == "NO" {
		def = "0"
	}

	if def != "" && ftype != "datetime" {
		def = "DEFAULT '" + def + "'"
	}

	if ftype == "text" ||
		ftype == "mediumtext" ||
		ftype == "longtext" ||
		ftype == "tinytext" ||
		ftype == "blob" ||
		ftype == "tinyblob" ||
		ftype == "mediumblob" ||
		ftype == "longblob" {
		def = ""
	}

	return def
}

func getString(d interface{}, k string) string {
	switch v := d.(map[string]interface{})[k].(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	case bool:
		if v == true {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}
func getAsString(d interface{}) string {
	switch v := d.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	case bool:
		if v == true {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}
