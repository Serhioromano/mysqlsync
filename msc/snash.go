package msc

import (
	"fmt"
	"os"
	"strings"
)

// Snash make DB model snapshot and saves to DBML file
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

	// Write header
	f.WriteString(fmt.Sprintf("// Schema: %s\n", db.Scheme))
	f.WriteString(fmt.Sprintf("// Prefix: %s\n", db.Prefix))
	f.WriteString("\n")

	for _, table := range tables {
		tableName := strings.Replace(table.Name, p.Prefix, "", 1)
		escapedTableName := escapeDBMLName(tableName)

		f.WriteString(fmt.Sprintf("Table \"%s\" {\n", escapedTableName))

		// Get fields
		fields, err := db.GetFields(table.Name)
		if err != nil {
			return err
		}

		// Get indexes
		indexes, err := db.GetIndexes(table.Name)
		if err != nil {
			return err
		}

		// Find primary key
		primaryKey := ""
		for _, index := range indexes {
			if index.Key_name == "PRIMARY" {
				primaryKey = index.Column_name
				break
			}
		}

		// Write fields
		for _, field := range fields {
			escapedColName := escapeDBMLName(field.COLUMN_NAME)

			settings := []string{}

			// Primary key
			if field.COLUMN_NAME == primaryKey {
				settings = append(settings, "pk")
				if strings.Contains(field.EXTRA, "auto_increment") {
					settings = append(settings, "increment")
				}
			}

			// Nullable
			if field.IS_NULLABLE == "NO" {
				settings = append(settings, "not null")
			} else {
				settings = append(settings, "null")
			}

			// Check for unique key on this column (non-primary, non-fk)
			for _, index := range indexes {
				if index.Key_name != "PRIMARY" &&
					!strings.Contains(index.Key_name, "fk_") &&
					index.Non_unique == "0" &&
					index.Column_name == field.COLUMN_NAME {
					// Count columns in this unique index
					colCount := 0
					for _, idx2 := range indexes {
						if idx2.Key_name == index.Key_name {
							colCount++
						}
					}
					if colCount == 1 {
						settings = append(settings, "unique")
					}
				}
			}

			// Default value
			if field.COLUMN_DEFAULT != nil && *field.COLUMN_DEFAULT != "" {
				def := *field.COLUMN_DEFAULT
				if field.DATA_TYPE == "int" || field.DATA_TYPE == "bigint" ||
					field.DATA_TYPE == "smallint" || field.DATA_TYPE == "tinyint" ||
					field.DATA_TYPE == "mediumint" || field.DATA_TYPE == "float" ||
					field.DATA_TYPE == "double" || field.DATA_TYPE == "decimal" {
					settings = append(settings, fmt.Sprintf("default: %s", def))
				} else {
					settings = append(settings, fmt.Sprintf("default: `%s`", strings.ReplaceAll(def, "`", "\\`")))
				}
			}

			// Comment / note
			if field.COLUMN_COMMENT != "" {
				settings = append(settings, fmt.Sprintf("note: '%s'", strings.ReplaceAll(field.COLUMN_COMMENT, "'", "\\'")))
			}

			settingsStr := ""
			if len(settings) > 0 {
				settingsStr = " [" + strings.Join(settings, ", ") + "]"
			}

			f.WriteString(fmt.Sprintf("  \"%s\" %s%s\n",
				escapedColName,
				field.COLUMN_TYPE,
				settingsStr))
		}

		// Write Indexes block (non-PK, non-FK, non-single-column-unique)
		indexBlockEntries := []string{}
		writtenIndexes := make(map[string]bool)

		for _, index := range indexes {
			if index.Key_name == "PRIMARY" || strings.Contains(index.Key_name, "fk_") {
				continue
			}
			if writtenIndexes[index.Key_name] {
				continue
			}
			writtenIndexes[index.Key_name] = true

			// Collect all columns for this index
			var indexCols []string
			for _, idx2 := range indexes {
				if idx2.Key_name == index.Key_name {
					indexCols = append(indexCols, escapeDBMLName(idx2.Column_name))
				}
			}

			// Skip single-column unique indexes (already inline on column)
			if index.Non_unique == "0" && len(indexCols) == 1 {
				continue
			}

			indexSettings := []string{}
			indexSettings = append(indexSettings, fmt.Sprintf("name: \"%s\"", index.Key_name))

			if index.Index_type == "FULLTEXT" {
				indexSettings = append(indexSettings, "type: fulltext")
			} else if index.Index_type == "BTREE" {
				indexSettings = append(indexSettings, "type: btree")
			} else if index.Index_type == "HASH" {
				indexSettings = append(indexSettings, "type: hash")
			}

			if index.Non_unique == "0" {
				indexSettings = append(indexSettings, "unique")
			}

			entry := fmt.Sprintf("    (%s) [%s]",
				strings.Join(indexCols, ", "),
				strings.Join(indexSettings, ", "))
			indexBlockEntries = append(indexBlockEntries, entry)
		}

		if len(indexBlockEntries) > 0 {
			f.WriteString("\n  Indexes {\n")
			for _, entry := range indexBlockEntries {
				f.WriteString(entry + "\n")
			}
			f.WriteString("  }\n")
		}

		// Write table-level metadata as a Note (engine, collation, comment)
		noteParts := []string{}
		if table.Engine != "" {
			noteParts = append(noteParts, fmt.Sprintf("Engine: %s", table.Engine))
		}
		if table.Collation != "" {
			noteParts = append(noteParts, fmt.Sprintf("Collation: %s", table.Collation))
		}
		if table.Comment != "" {
			noteParts = append(noteParts, fmt.Sprintf("Comment: %s", table.Comment))
		}
		if len(noteParts) > 0 {
			f.WriteString(fmt.Sprintf("\n  Note: '''%s'''\n", strings.Join(noteParts, " | ")))
		}

		f.WriteString("}\n\n")
	}

	// Write References (foreign keys)
	for _, table := range tables {
		tableName := strings.Replace(table.Name, p.Prefix, "", 1)

		constraines, err := db.GetConstraines(table.Name)
		if err != nil {
			return err
		}
		if len(constraines) == 0 {
			continue
		}

		for _, constrain := range constraines {
			refTableName := strings.Replace(constrain.REFERENCED_TABLE_NAME, p.Prefix, "", 1)

			refSettings := []string{}
			if constrain.DELETE_RULE != "" && strings.ToUpper(constrain.DELETE_RULE) != "NO ACTION" {
				deleteRule := strings.ToLower(constrain.DELETE_RULE)
				refSettings = append(refSettings, fmt.Sprintf("delete: %s", deleteRule))
			}
			if constrain.UPDATE_RULE != "" && strings.ToUpper(constrain.UPDATE_RULE) != "NO ACTION" {
				updateRule := strings.ToLower(constrain.UPDATE_RULE)
				refSettings = append(refSettings, fmt.Sprintf("update: %s", updateRule))
			}

			refSettingsStr := ""
			if len(refSettings) > 0 {
				refSettingsStr = " [" + strings.Join(refSettings, ", ") + "]"
			}

			f.WriteString(fmt.Sprintf("Ref: \"%s\".\"%s\" > \"%s\".\"%s\"%s\n",
				escapeDBMLName(tableName),
				escapeDBMLName(constrain.COLUMN_NAME),
				escapeDBMLName(refTableName),
				escapeDBMLName(constrain.REFERENCED_COLUMN_NAME),
				refSettingsStr))
		}
	}

	return nil
}

// escapeDBMLName escapes a name for DBML double-quoted identifiers.
// If the name contains special characters, it's returned as-is (already escaped).
func escapeDBMLName(name string) string {
	// DBML names in double quotes just need backslash escaping of quotes
	return strings.ReplaceAll(name, "\"", "\\\"")
}
