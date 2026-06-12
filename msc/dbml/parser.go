package dbml

import (
	"fmt"
	"strings"

	s "github.com/serhioromano/mysqlsync/msc/schema"
)

// Parse parses a DBML document string and returns a Schema.
func Parse(input string) (*s.Schema, error) {
	schema := &s.Schema{}

	lines := strings.Split(input, "\n")

	// Collect Ref lines for later attachment
	type refEntry struct {
		fromTable  string
		fromCol    string
		toTable    string
		toCol      string
		deleteRule string
		updateRule string
	}
	var refs []refEntry

	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Header comments
		if strings.HasPrefix(line, "//") {
			comment := strings.TrimSpace(line[2:])
			if strings.HasPrefix(comment, "Schema:") {
				schema.Name = strings.TrimSpace(comment[7:])
			} else if strings.HasPrefix(comment, "Prefix:") {
				schema.Prefix = strings.TrimSpace(comment[7:])
			}
			i++
			continue
		}

		if line == "" {
			i++
			continue
		}

		// Table definition
		if strings.HasPrefix(line, "Table ") {
			table, consumed, err := parseTable(lines, i)
			if err != nil {
				return nil, err
			}
			schema.Tables = append(schema.Tables, table)
			i = consumed
			continue
		}

		// Ref line
		if strings.HasPrefix(line, "Ref:") {
			ref, err := parseRefLine(line)
			if err != nil {
				return nil, fmt.Errorf("error parsing Ref at line %d: %v", i+1, err)
			}
			refs = append(refs, ref)
			i++
			continue
		}

		i++
	}

	// Attach refs as constraints to their source tables
	for _, ref := range refs {
		for idx := range schema.Tables {
			if schema.Tables[idx].Name == ref.fromTable {
				constraintName := fmt.Sprintf("fk_%s_%s", ref.fromTable, ref.fromCol)
				schema.Tables[idx].Constraints = append(schema.Tables[idx].Constraints, s.ConstraintDef{
					Name:              constraintName,
					ColumnName:        ref.fromCol,
					RefTableName:      ref.toTable,
					RefColumnName:     ref.toCol,
					UpdateRule:        strings.ToUpper(ref.updateRule),
					DeleteRule:        strings.ToUpper(ref.deleteRule),
				})
			}
		}
	}

	return schema, nil
}

func parseTable(lines []string, start int) (s.TableDef, int, error) {
	table := s.TableDef{
		Engine:    "InnoDB",
		Collation: "utf8_general_ci",
	}

	headerLine := strings.TrimSpace(lines[start])
	rest := strings.TrimPrefix(headerLine, "Table ")
	rest = strings.TrimSpace(rest)

	qStart := strings.Index(rest, "\"")
	if qStart == -1 {
		return table, start, fmt.Errorf("expected quoted table name at line %d: %s", start+1, headerLine)
	}
	qEnd := strings.Index(rest[qStart+1:], "\"")
	if qEnd == -1 {
		return table, start, fmt.Errorf("unterminated table name at line %d: %s", start+1, headerLine)
	}
	table.Name = rest[qStart+1 : qStart+1+qEnd]

	i := start + 1
	inIndexes := false
	inNote := false
	noteLines := []string{}

	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Handle Note block (multi-line)
		if inNote {
			if strings.Contains(trimmed, "'''") {
				endIdx := strings.Index(trimmed, "'''")
				if endIdx > 0 {
					noteLines = append(noteLines, trimmed[:endIdx])
				}
				parseTableNote(&table, strings.Join(noteLines, "\n"))
				inNote = false
				noteLines = nil
				i++
				continue
			}
			noteLines = append(noteLines, trimmed)
			i++
			continue
		}

		if trimmed == "}" {
			i++
			break
		}

		if strings.HasPrefix(trimmed, "Indexes {") || trimmed == "Indexes {" {
			inIndexes = true
			i++
			continue
		}

		if inIndexes && trimmed == "}" {
			inIndexes = false
			i++
			continue
		}

		if inIndexes {
			idxDef, err := parseIndexEntry(trimmed)
			if err == nil {
				table.Indexes = append(table.Indexes, idxDef)
			}
			i++
			continue
		}

		// Note block start
		if strings.HasPrefix(trimmed, "Note:") && strings.Contains(trimmed, "'''") {
			inNote = true
			afterNote := trimmed[strings.Index(trimmed, "'''")+3:]
			if strings.Contains(afterNote, "'''") {
				endIdx := strings.Index(afterNote, "'''")
				parseTableNote(&table, strings.TrimSpace(afterNote[:endIdx]))
				inNote = false
				i++
				continue
			}
			if afterNote != "" {
				noteLines = append(noteLines, afterNote)
			}
			i++
			continue
		}

		// Column definition
		if strings.HasPrefix(trimmed, "\"") {
			fd, err := parseColumnDef(trimmed)
			if err == nil {
				if fd.IsPrimary {
					table.PrimaryKey = fd.Name
					// Add PK index
					table.Indexes = append(table.Indexes, s.IndexDef{
						Name:      "PRIMARY",
						Columns:   []string{fd.Name},
						IsUnique:  true,
						IndexType: "BTREE",
					})
				}
				if fd.IsUnique {
					// Already captured inline as unique index; add if not PK
					if !fd.IsPrimary {
						table.Indexes = append(table.Indexes, s.IndexDef{
							Name:      fd.Name,
							Columns:   []string{fd.Name},
							IsUnique:  true,
							IndexType: "BTREE",
						})
					}
				}
				table.Fields = append(table.Fields, fd)
			}
			i++
			continue
		}

		i++
	}

	return table, i, nil
}

func parseColumnDef(line string) (s.FieldDef, error) {
	fd := s.FieldDef{IsNullable: true}

	trimmed := strings.TrimSpace(line)

	qStart := strings.Index(trimmed, "\"")
	if qStart == -1 {
		return fd, fmt.Errorf("expected quoted field name in: %s", trimmed)
	}
	qEnd := strings.Index(trimmed[qStart+1:], "\"")
	if qEnd == -1 {
		return fd, fmt.Errorf("unterminated field name in: %s", trimmed)
	}
	fd.Name = trimmed[qStart+1 : qStart+1+qEnd]

	rest := trimmed[qStart+1+qEnd+1:]
	rest = strings.TrimSpace(rest)

	var colType string
	var settings []string

	bracketIdx := strings.Index(rest, "[")
	if bracketIdx >= 0 && strings.HasSuffix(strings.TrimSpace(rest), "]") {
		colType = strings.TrimSpace(rest[:bracketIdx])
		settings = parseSettings(rest[bracketIdx+1 : len(rest)-1])
	} else {
		colType = strings.TrimSpace(rest)
	}

	fd.ColumnType = colType
	fd.DataType = extractDataType(colType)

	for _, s := range settings {
		switch {
		case s == "pk":
			fd.IsPrimary = true
		case s == "increment":
			fd.IsAutoIncr = true
		case s == "not null":
			fd.IsNullable = false
		case s == "null":
			fd.IsNullable = true
		case s == "unique":
			fd.IsUnique = true
		case strings.HasPrefix(s, "default:"):
			defVal := strings.TrimSpace(s[8:])
			if len(defVal) >= 2 && defVal[0] == '`' && defVal[len(defVal)-1] == '`' {
				defVal = defVal[1 : len(defVal)-1]
				defVal = strings.ReplaceAll(defVal, "\\`", "`")
			}
			fd.DefaultValue = &defVal
		case strings.HasPrefix(s, "note:"):
			comment := strings.TrimSpace(s[5:])
			if len(comment) >= 2 && comment[0] == '\'' && comment[len(comment)-1] == '\'' {
				comment = comment[1 : len(comment)-1]
				comment = strings.ReplaceAll(comment, "\\'", "'")
			}
			fd.Comment = comment
		}
	}

	return fd, nil
}

func parseIndexEntry(line string) (s.IndexDef, error) {
	idx := s.IndexDef{IndexType: "BTREE"}

	trimmed := strings.TrimSpace(line)

	parenStart := strings.Index(trimmed, "(")
	parenEnd := strings.Index(trimmed, ")")
	if parenStart == -1 || parenEnd == -1 {
		return idx, fmt.Errorf("invalid index entry: %s", trimmed)
	}

	colsStr := trimmed[parenStart+1 : parenEnd]
	for _, col := range strings.Split(colsStr, ",") {
		col = strings.TrimSpace(col)
		col = strings.Trim(col, "\"")
		if col != "" {
			idx.Columns = append(idx.Columns, col)
		}
	}

	settingsStr := ""
	bracketStart := strings.Index(trimmed[parenEnd:], "[")
	if bracketStart >= 0 {
		bracketEnd := strings.LastIndex(trimmed, "]")
		if bracketEnd > parenEnd {
			settingsStr = trimmed[parenEnd+bracketStart+1 : bracketEnd]
		}
	}

	for _, s := range parseSettings(settingsStr) {
		s = strings.TrimSpace(s)
		switch {
		case strings.HasPrefix(s, "name:"):
			idx.Name = strings.Trim(strings.TrimSpace(s[5:]), "\"")
		case strings.HasPrefix(s, "type:"):
			t := strings.ToUpper(strings.TrimSpace(s[5:]))
			if t == "FULLTEXT" || t == "HASH" {
				idx.IndexType = t
			}
		case s == "unique":
			idx.IsUnique = true
		}
	}

	if idx.Name == "" && len(idx.Columns) > 0 {
		idx.Name = idx.Columns[0]
	}

	return idx, nil
}

func parseRefLine(line string) (struct {
	fromTable  string
	fromCol    string
	toTable    string
	toCol      string
	deleteRule string
	updateRule string
}, error) {
	var ref struct {
		fromTable  string
		fromCol    string
		toTable    string
		toCol      string
		deleteRule string
		updateRule string
	}
	ref.deleteRule = "NO ACTION"
	ref.updateRule = "NO ACTION"

	trimmed := strings.TrimSpace(strings.TrimPrefix(line, "Ref:"))
	parts := strings.Split(trimmed, " > ")
	if len(parts) < 2 {
		return ref, fmt.Errorf("invalid Ref format: %s", line)
	}

	// Left side
	left := strings.Split(strings.TrimSpace(parts[0]), ".")
	if len(left) != 2 {
		return ref, fmt.Errorf("invalid left side of Ref: %s", parts[0])
	}
	ref.fromTable = strings.Trim(left[0], "\"")
	ref.fromCol = strings.Trim(left[1], "\"")

	// Right side with optional settings
	right := strings.TrimSpace(parts[1])
	rightTable, rightCol := "", ""
	var refSettings string

	bracketIdx := strings.Index(right, "[")
	if bracketIdx >= 0 && strings.HasSuffix(strings.TrimSpace(right), "]") {
		rightParts := strings.Split(strings.TrimSpace(right[:bracketIdx]), ".")
		if len(rightParts) != 2 {
			return ref, fmt.Errorf("invalid right side of Ref: %s", right[:bracketIdx])
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

	for _, s := range parseSettings(refSettings) {
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, "delete:") {
			ref.deleteRule = strings.TrimSpace(s[7:])
		} else if strings.HasPrefix(s, "update:") {
			ref.updateRule = strings.TrimSpace(s[7:])
		}
	}

	return ref, nil
}

func parseTableNote(table *s.TableDef, noteText string) {
	for _, part := range strings.Split(noteText, "|") {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "Engine:"):
			table.Engine = strings.TrimSpace(part[7:])
		case strings.HasPrefix(part, "Collation:"):
			table.Collation = strings.TrimSpace(part[10:])
		case strings.HasPrefix(part, "Comment:"):
			table.Comment = strings.TrimSpace(part[8:])
		}
	}
}

// parseSettings splits comma-separated settings, respecting quotes.
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

func extractDataType(colType string) string {
	paren := strings.Index(colType, "(")
	if paren >= 0 {
		return strings.ToLower(strings.TrimSpace(colType[:paren]))
	}
	space := strings.Index(colType, " ")
	if space >= 0 {
		return strings.ToLower(strings.TrimSpace(colType[:space]))
	}
	return strings.ToLower(strings.TrimSpace(colType))
}
