package msc

import (
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

// refEntry holds a parsed foreign key reference from DBML
type refEntry struct {
	fromTable  string
	fromCol    string
	toTable    string
	toCol      string
	deleteRule string
	updateRule string
}

// Run DB model from file
func (r *Restore) Run(p Config) error {
	path := p.FilesPath
	file := p.File

	dat, err := ioutil.ReadFile(path + "/" + file)
	if err != nil {
		return err
	}

	data, err := parseDBML(string(dat))
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

	r.runSQL(true, "USE "+db.Scheme)
	r.runSQL(false, "SET @@session.sql_mode =\"ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER,NO_ENGINE_SUBSTITUTION\"")

	currentTables, err := db.GetTables()
	if err != nil {
		return err
	}
	importTables := data["tables"]
	for _, importTable := range importTables.(map[string]interface{}) {
		newTableName := db.Prefix + getString(importTable, "Name")

		if currentTable, ok := importTableInCurrent(currentTables, newTableName); ok == true {
			// Update table
			if currentTable.Collation != getString(importTable, "Collation") ||
				currentTable.Engine != getString(importTable, "Engine") ||
				currentTable.Comment != getString(importTable, "Comment") {
				r.runSQL(true, fmt.Sprintf(
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
						r.runSQL(true, fmt.Sprintf(
							`ALTER TABLE `+"`%s`"+` CHANGE COLUMN `+"`%s`"+` %s`,
							newTableName, getString(importField, "COLUMN_NAME"), ufs))
					}
				} else {
					// Create field
					r.runSQL(true, fmt.Sprintf(`ALTER TABLE `+"`%s`"+` ADD COLUMN %s`,
						newTableName, ufs))
				}

			}

			if p.DColumn {
				for _, currentField := range currentFields {
					if !currentFieldInImport(importFields, currentField.COLUMN_NAME) {
						r.runSQL(true, fmt.Sprintf(`ALTER TABLE `+"`%s`"+` DROP COLUMN  `+"`%s`",
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

				r.runSQL(true, fmt.Sprintf("ALTER TABLE `%s` ADD INDEX `%s` (%s ASC)",
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
						r.runSQL(false, fmt.Sprintf("DROP INDEX `%s` ON %s",
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

			r.runSQL(true, fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s` (\n\t%s\n) ENGINE=%s COLLATE=%s COMMENT='%s'",
				newTableName,
				strings.Join(sql, ",\n\t"),
				getString(importTable, "Engine"),
				getString(importTable, "Collation"),
				getString(importTable, "Comment")))

		}
	}

	if p.DTable {
		for _, currentTable := range currentTables {
			if !currentTableInImport(p.Prefix, importTables, currentTable.Name) {
				r.runSQL(true, fmt.Sprintf("DROP TABLE IF EXISTS `%s`", currentTable.Name))
			}
		}
	}

	// Constrains
	r.runSQL(false, "SET @@session.UNIQUE_CHECKS=0")
	r.runSQL(false, "SET @@session.FOREIGN_KEY_CHECKS=0")
	r.runSQL(false, "SET @@session.sql_mode='ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION'")
	currentTables, err = db.GetTables()
	if err != nil {
		return err
	}
	for _, importTable := range data["tables"].(map[string]interface{}) {
		newTableName := db.Prefix + getString(importTable, "Name")
		currentConstrains, err := db.GetConstraines((getString(importTable, "Name")))
		if err != nil {
			return err
		}
		importConstrains := importTable.(map[string]interface{})["constraines"].(map[string]interface{})

		for _, currentConstrain := range currentConstrains {
			if currentConstrainInImport(importConstrains, currentConstrain.CONSTRAINT_NAME, getString(data, "prefix"), p.Prefix) {
				continue
			}
			if p.DConstraint == false {
				continue
			}
			r.runSQL(false, fmt.Sprintf("ALTER TABLE `%s` DROP FOREIGN KEY `%s`",
				newTableName, currentConstrain.CONSTRAINT_NAME))
		}
		for _, importConstrain := range importConstrains {
			if importConstrainInCurrent(currentConstrains, getString(importConstrain, "CONSTRAINT_NAME"), getString(data, "prefix"), p.Prefix) {
				continue
			}

			importTableName := p.Prefix + strings.Replace(getString(importConstrain, "REFERENCED_TABLE_NAME"), getString(data, "prefix"), "", 1)
			r.runSQL(true, fmt.Sprintf("ALTER TABLE `%s` ADD CONSTRAINT `%s` FOREIGN KEY (`%s`)"+
				" REFERENCES `%s` (`%s`) ON DELETE %s ON UPDATE %s",
				newTableName,
				p.Prefix+getString(importConstrain, "CONSTRAINT_NAME"),
				getString(importConstrain, "COLUMN_NAME"),
				importTableName,
				getString(importConstrain, "REFERENCED_COLUMN_NAME"),
				getString(importConstrain, "DELETE_RULE"),
				getString(importConstrain, "UPDATE_RULE")))
		}

		if p.Optimize && getString(importTable, "Engine") == "InnoDB" || getString(importTable, "Engine") == "MyISAM" {
			r.runSQL(false, "OPTIMIZE TABLE "+newTableName)
		}

	}

	return nil
}

func (r *Restore) runSQL(p bool, sql string) error {
	color.Cyan(sql)
	fmt.Println("")
	_, err := r.conn.Conn.Query(sql)
	if p == true && err != nil {
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

func currentConstrainInImport(i interface{}, name string, oldp string, newp string) bool {
	ok := false
	cname := strings.Replace(name, newp, "", 1)
	for _, f := range i.(map[string]interface{}) {
		iname := strings.Replace(getString(f, "CONSTRAINT_NAME"), oldp, "", 1)
		if cname == iname {
			ok = true
			break
		}
	}
	return ok
}
func importConstrainInCurrent(c []Constrain, name string, oldp string, newp string) bool {
	ok := false
	iname := strings.Replace(name, oldp, "", 1)
	for _, f := range c {
		cname := strings.Replace(f.CONSTRAINT_NAME, newp, "", 1)
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

// parseDBML parses a DBML document into the same map[string]interface{}
// structure that the old JSON format used, so the restore logic works unchanged.
func parseDBML(input string) (map[string]interface{}, error) {
	data := make(map[string]interface{})
	tables := make(map[string]interface{})

	lines := strings.Split(input, "\n")

	var schemaName string
	var schemaPrefix string

	// Collect all Ref lines for foreign keys
	var refs []refEntry

	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Header comments
		if strings.HasPrefix(line, "//") {
			comment := strings.TrimSpace(line[2:])
			if strings.HasPrefix(comment, "Schema:") {
				schemaName = strings.TrimSpace(comment[7:])
			} else if strings.HasPrefix(comment, "Prefix:") {
				schemaPrefix = strings.TrimSpace(comment[7:])
			}
			i++
			continue
		}

		// Empty line
		if line == "" {
			i++
			continue
		}

		// Table definition
		if strings.HasPrefix(line, "Table ") {
			tableData, consumed, err := parseTable(lines, i)
			if err != nil {
				return nil, err
			}
			tableName := getString(tableData, "Name")
			tables[tableName] = tableData
			i = consumed
			continue
		}

		// Ref line
		if strings.HasPrefix(line, "Ref:") {
			ref, err := parseRef(line)
			if err != nil {
				return nil, fmt.Errorf("error parsing Ref at line %d: %v", i+1, err)
			}
			refs = append(refs, ref)
			i++
			continue
		}

		i++
	}

	// Apply Refs to their respective tables as constraints
	for _, ref := range refs {
		fromTable, ok := tables[ref.fromTable]
		if !ok {
			continue
		}
		fromTableMap := fromTable.(map[string]interface{})
		constraines, ok := fromTableMap["constraines"].(map[string]interface{})
		if !ok {
			constraines = make(map[string]interface{})
			fromTableMap["constraines"] = constraines
		}

		// Generate a constraint name: fk_<fromTable>_<fromCol>
		constraintName := fmt.Sprintf("fk_%s_%s", ref.fromTable, ref.fromCol)
		constraines[constraintName] = map[string]interface{}{
			"CONSTRAINT_NAME":        constraintName,
			"COLUMN_NAME":            ref.fromCol,
			"REFERENCED_TABLE_NAME":  schemaPrefix + ref.toTable,
			"REFERENCED_COLUMN_NAME": ref.toCol,
			"UPDATE_RULE":            strings.ToUpper(ref.updateRule),
			"DELETE_RULE":            strings.ToUpper(ref.deleteRule),
		}
	}

	data["name"] = schemaName
	data["prefix"] = schemaPrefix
	data["tables"] = tables

	return data, nil
}

// parseTable parses a Table block starting at line index start.
// Returns the table data map, the next line index to process, and any error.
func parseTable(lines []string, start int) (map[string]interface{}, int, error) {
	tableData := make(map[string]interface{})

	// Parse: Table "name" {
	headerLine := strings.TrimSpace(lines[start])
	tableName := ""

	// Extract table name from: Table "name" {  or  Table "name" {
	rest := strings.TrimPrefix(headerLine, "Table ")
	rest = strings.TrimSpace(rest)
	// Find the opening quote
	qStart := strings.Index(rest, "\"")
	if qStart == -1 {
		return nil, start, fmt.Errorf("expected quoted table name at line %d: %s", start+1, headerLine)
	}
	qEnd := strings.Index(rest[qStart+1:], "\"")
	if qEnd == -1 {
		return nil, start, fmt.Errorf("unterminated table name at line %d: %s", start+1, headerLine)
	}
	tableName = rest[qStart+1 : qStart+1+qEnd]

	tableData["Name"] = tableName
	tableData["Engine"] = "InnoDB"
	tableData["Collation"] = "utf8_general_ci"
	tableData["Comment"] = ""

	fields := make(map[string]interface{})
	indexes := make(map[string]interface{})
	constraines := make(map[string]interface{})

	fieldOrdinal := 1
	i := start + 1
	inIndexes := false
	inNote := false
	noteLines := []string{}

	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Handle Note block
		if inNote {
			if strings.Contains(trimmed, "'''") {
				// End of note
				endIdx := strings.Index(trimmed, "'''")
				if endIdx > 0 {
					noteLines = append(noteLines, trimmed[:endIdx])
				}
				noteText := strings.Join(noteLines, "\n")
				noteText = strings.TrimSpace(noteText)
				// Parse engine/collation/comment from note
				parseTableNote(tableData, noteText)
				inNote = false
				noteLines = nil
				i++
				continue
			}
			noteLines = append(noteLines, trimmed)
			i++
			continue
		}

		// End of table block
		if trimmed == "}" {
			i++
			break
		}

		// Start of Indexes block
		if strings.HasPrefix(trimmed, "Indexes {") || trimmed == "Indexes {" {
			inIndexes = true
			i++
			continue
		}

		// End of Indexes block
		if inIndexes && trimmed == "}" {
			inIndexes = false
			i++
			continue
		}

		// Inside Indexes block
		if inIndexes {
			idxData, err := parseIndexEntry(trimmed)
			if err == nil {
				indexes[idxData["Key_name"].(string)] = idxData
			}
			i++
			continue
		}

		// Note block start
		if strings.HasPrefix(trimmed, "Note:") && strings.Contains(trimmed, "'''") {
			inNote = true
			// Check if note starts and ends on same line
			afterNote := trimmed[strings.Index(trimmed, "'''")+3:]
			if strings.Contains(afterNote, "'''") {
				endIdx := strings.Index(afterNote, "'''")
				noteText := strings.TrimSpace(afterNote[:endIdx])
				parseTableNote(tableData, noteText)
				inNote = false
				i++
				continue
			}
			// Multi-line note: capture content after opening '''
			firstContent := afterNote
			if firstContent != "" {
				noteLines = append(noteLines, firstContent)
			}
			i++
			continue
		}

		// Column definition: "name" type [settings]
		if strings.HasPrefix(trimmed, "\"") {
			fieldData, err := parseColumnDef(trimmed, fieldOrdinal)
			if err == nil {
				fieldName := getString(fieldData, "COLUMN_NAME")

				// Check if this is a primary key
				if fieldData["COLUMN_KEY"] == "PRI" {
					tableData["Primary"] = fieldName

					// Add primary key index
					indexes["PRIMARY"] = map[string]interface{}{
						"Key_name":    "PRIMARY",
						"Non_unique":  "0",
						"Index_type":  "BTREE",
						"fields":      []interface{}{fieldName},
						"Seq_in_index": "1",
						"Column_name":  fieldName,
						"Null":         "",
						"Comment":      "",
						"Index_comment": "",
					}
				}

				// Check if column has unique setting
				if settings, ok := fieldData["_settings"].([]string); ok {
					for _, s := range settings {
						if s == "unique" {
							uniqueName := fieldName
							indexes[uniqueName] = map[string]interface{}{
								"Key_name":    uniqueName,
								"Non_unique":  "0",
								"Index_type":  "BTREE",
								"fields":      []interface{}{fieldName},
								"Seq_in_index": "1",
								"Column_name":  fieldName,
								"Null":         "",
								"Comment":      "",
								"Index_comment": "",
							}
						}
					}
					delete(fieldData, "_settings")
				}

				fields[fmt.Sprintf("%d", fieldOrdinal)] = fieldData
				fieldOrdinal++
			}
			i++
			continue
		}

		i++
	}

	tableData["fields"] = fields
	tableData["indexes"] = indexes
	tableData["constraines"] = constraines

	return tableData, i, nil
}

// parseColumnDef parses a DBML column definition like:
//
//	"name" type [pk, increment, not null, default: `value`, note: 'comment']
func parseColumnDef(line string, ordinal int) (map[string]interface{}, error) {
	field := make(map[string]interface{})

	trimmed := strings.TrimSpace(line)

	// Extract field name: first quoted string
	qStart := strings.Index(trimmed, "\"")
	if qStart == -1 {
		return nil, fmt.Errorf("expected quoted field name in: %s", trimmed)
	}
	qEnd := strings.Index(trimmed[qStart+1:], "\"")
	if qEnd == -1 {
		return nil, fmt.Errorf("unterminated field name in: %s", trimmed)
	}
	fieldName := trimmed[qStart+1 : qStart+1+qEnd]

	// Everything after the closing quote up to [ or end
	rest := trimmed[qStart+1+qEnd+1:]
	rest = strings.TrimSpace(rest)

	// Extract column type and settings
	colType := ""
	var settings []string

	bracketIdx := strings.Index(rest, "[")
	if bracketIdx >= 0 && strings.HasSuffix(strings.TrimSpace(rest), "]") {
		colType = strings.TrimSpace(rest[:bracketIdx])
		settingsStr := rest[bracketIdx+1 : len(rest)-1]
		settings = parseSettings(settingsStr)
	} else {
		colType = strings.TrimSpace(rest)
	}

	field["ORDINAL_POSITION"] = fmt.Sprintf("%d", ordinal)
	field["COLUMN_NAME"] = fieldName
	field["COLUMN_TYPE"] = colType
	field["COLUMN_DEFAULT"] = nil
	field["COLUMN_COMMENT"] = ""
	field["IS_NULLABLE"] = "YES"
	field["DATA_TYPE"] = extractDataType(colType)
	field["EXTRA"] = ""
	field["COLUMN_KEY"] = ""
	field["_settings"] = settings

	for _, s := range settings {
		switch {
		case s == "pk":
			field["COLUMN_KEY"] = "PRI"
		case s == "increment":
			field["EXTRA"] = "auto_increment"
		case s == "not null":
			field["IS_NULLABLE"] = "NO"
		case s == "null":
			field["IS_NULLABLE"] = "YES"
		case strings.HasPrefix(s, "default:"):
			defVal := strings.TrimSpace(s[8:])
			// Remove backtick quoting
			if len(defVal) >= 2 && defVal[0] == '`' && defVal[len(defVal)-1] == '`' {
				defVal = defVal[1 : len(defVal)-1]
				defVal = strings.ReplaceAll(defVal, "\\`", "`")
			}
			field["COLUMN_DEFAULT"] = &defVal
		case strings.HasPrefix(s, "note:"):
			comment := strings.TrimSpace(s[5:])
			// Remove single quotes
			if len(comment) >= 2 && comment[0] == '\'' && comment[len(comment)-1] == '\'' {
				comment = comment[1 : len(comment)-1]
				comment = strings.ReplaceAll(comment, "\\'", "'")
			}
			field["COLUMN_COMMENT"] = comment
		}
	}

	return field, nil
}

// parseSettings parses the comma-separated settings inside brackets.
// Handles values with colons, quoted strings, etc.
func parseSettings(s string) []string {
	var result []string
	var current strings.Builder
	inBacktick := false
	inSingleQuote := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '`' && !inSingleQuote {
			inBacktick = !inBacktick
			current.WriteByte(ch)
		} else if ch == '\'' && !inBacktick {
			inSingleQuote = !inSingleQuote
			current.WriteByte(ch)
		} else if ch == ',' && !inBacktick && !inSingleQuote {
			result = append(result, strings.TrimSpace(current.String()))
			current.Reset()
		} else {
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		result = append(result, strings.TrimSpace(current.String()))
	}

	return result
}

// parseIndexEntry parses an index entry inside an Indexes block:
//
//	(col1, col2) [name: "idx_name", type: btree, unique]
func parseIndexEntry(line string) (map[string]interface{}, error) {
	idx := make(map[string]interface{})

	trimmed := strings.TrimSpace(line)

	// Find the parenthesized column list
	parenStart := strings.Index(trimmed, "(")
	parenEnd := strings.Index(trimmed, ")")
	if parenStart == -1 || parenEnd == -1 {
		return nil, fmt.Errorf("invalid index entry: %s", trimmed)
	}

	colsStr := trimmed[parenStart+1 : parenEnd]
	var cols []interface{}
	for _, col := range strings.Split(colsStr, ",") {
		col = strings.TrimSpace(col)
		col = strings.Trim(col, "\"")
		if col != "" {
			cols = append(cols, col)
		}
	}

	// Parse settings
	settingsStr := ""
	bracketStart := strings.Index(trimmed[parenEnd:], "[")
	if bracketStart >= 0 {
		bracketEnd := strings.LastIndex(trimmed, "]")
		if bracketEnd > parenEnd {
			settingsStr = trimmed[parenEnd+bracketStart+1 : bracketEnd]
		}
	}

	settings := parseSettings(settingsStr)
	indexName := ""
	indexType := "BTREE"
	nonUnique := "1"

	for _, s := range settings {
		s = strings.TrimSpace(s)
		switch {
		case strings.HasPrefix(s, "name:"):
			name := strings.TrimSpace(s[5:])
			name = strings.Trim(name, "\"")
			indexName = name
		case strings.HasPrefix(s, "type:"):
			t := strings.TrimSpace(s[5:])
			if strings.ToUpper(t) == "FULLTEXT" {
				indexType = "FULLTEXT"
			} else if strings.ToUpper(t) == "HASH" {
				indexType = "HASH"
			} else {
				indexType = "BTREE"
			}
		case s == "unique":
			nonUnique = "0"
		}
	}

	if indexName == "" && len(cols) > 0 {
		indexName = cols[0].(string)
	}

	idx["Key_name"] = indexName
	idx["Non_unique"] = nonUnique
	idx["Index_type"] = indexType
	idx["fields"] = cols
	if len(cols) > 0 {
		idx["Column_name"] = cols[0].(string)
	}
	idx["Seq_in_index"] = "1"
	idx["Collation"] = nil
	idx["Cardinality"] = nil
	idx["Sub_part"] = nil
	idx["Packed"] = nil
	idx["Null"] = ""
	idx["Comment"] = ""
	idx["Index_comment"] = ""

	return idx, nil
}

// parseRef parses a Ref line:
//
//	Ref: "table1"."col1" > "table2"."col2" [delete: cascade, update: no action]
func parseRef(line string) (refEntry, error) {
	var ref refEntry
	ref.deleteRule = "NO ACTION"
	ref.updateRule = "NO ACTION"

	trimmed := strings.TrimSpace(line)
	// Remove "Ref:" prefix
	rest := strings.TrimPrefix(trimmed, "Ref:")
	rest = strings.TrimSpace(rest)

	// Split on " > "
	parts := strings.Split(rest, " > ")
	if len(parts) < 2 {
		return ref, fmt.Errorf("invalid Ref format, expected '>' separator: %s", line)
	}

	// Parse left side: "table"."col"
	left := strings.TrimSpace(parts[0])
	leftParts := strings.Split(left, ".")
	if len(leftParts) != 2 {
		return ref, fmt.Errorf("invalid left side of Ref: %s", left)
	}
	ref.fromTable = strings.Trim(leftParts[0], "\"")
	ref.fromCol = strings.Trim(leftParts[1], "\"")

	// Parse right side: "table"."col" [settings]
	right := strings.TrimSpace(parts[1])

	rightTable := ""
	rightCol := ""
	var refSettings string

	bracketIdx := strings.Index(right, "[")
	if bracketIdx >= 0 && strings.HasSuffix(strings.TrimSpace(right), "]") {
		rightWithoutSettings := strings.TrimSpace(right[:bracketIdx])
		rightParts := strings.Split(rightWithoutSettings, ".")
		if len(rightParts) != 2 {
			return ref, fmt.Errorf("invalid right side of Ref: %s", rightWithoutSettings)
		}
		rightTable = strings.Trim(rightParts[0], "\"")
		rightCol = strings.Trim(rightParts[1], "\"")
		refSettings = right[bracketIdx+1 : len(right)-1]
	} else {
		rightParts := strings.Split(right, ".")
		if len(rightParts) != 2 {
			return ref, fmt.Errorf("invalid right side of Ref: %s", right)
		}
		rightTable = strings.Trim(rightParts[0], "\"")
		rightCol = strings.Trim(rightParts[1], "\"")
	}

	ref.toTable = rightTable
	ref.toCol = rightCol

	// Parse reference settings
	if refSettings != "" {
		settings := parseSettings(refSettings)
		for _, s := range settings {
			s = strings.TrimSpace(s)
			if strings.HasPrefix(s, "delete:") {
				ref.deleteRule = strings.TrimSpace(s[7:])
			} else if strings.HasPrefix(s, "update:") {
				ref.updateRule = strings.TrimSpace(s[7:])
			}
		}
	}

	return ref, nil
}

// parseTableNote parses the Note content of a table and extracts
// Engine, Collation, and Comment metadata.
func parseTableNote(tableData map[string]interface{}, noteText string) {
	parts := strings.Split(noteText, "|")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "Engine:") {
			tableData["Engine"] = strings.TrimSpace(part[7:])
		} else if strings.HasPrefix(part, "Collation:") {
			tableData["Collation"] = strings.TrimSpace(part[10:])
		} else if strings.HasPrefix(part, "Comment:") {
			tableData["Comment"] = strings.TrimSpace(part[8:])
		}
	}
}

// extractDataType extracts the base data type from a column type like "varchar(255)" or "int(11)".
func extractDataType(colType string) string {
	paren := strings.Index(colType, "(")
	if paren >= 0 {
		return strings.ToLower(colType[:paren])
	}
	// Handle types with spaces like "unsigned int"
	space := strings.Index(colType, " ")
	if space >= 0 {
		return strings.ToLower(colType[:space])
	}
	return strings.ToLower(colType)
}
