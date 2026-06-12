# AGENTS.md — Instructions for AI Coding Agents

This file provides guidelines for AI agents (like pi, Claude, Copilot, etc.) working on the mysqlsync codebase.

## Project Identity

- **Language:** Go 1.16
- **Module:** `github.com/serhioromano/mysqlsync`
- **Purpose:** MySQL schema snapshot → JSON → diff-driven restore/migration tool
- **Entry point:** `main.go`
- **License:** Apache-style (see LICENSE)

## Coding Conventions

1. **Follow existing patterns** — The codebase has a consistent style: exported types/functions use PascalCase, unexported use camelCase.
2. **Error handling** — Functions return `(result, error)`. The restore command uses `panic()` in some places for simplicity; prefer returning errors in new code.
3. **Package structure** — CLI logic lives in `cmd/`, core library logic lives in `msc/`. Keep them separate.
4. **JSON tags** — All struct fields intended for serialization must have `json:"..."` tags matching the existing naming convention (e.g., `COLUMN_NAME`, `Key_name` — mixed case per MySQL's INFORMATION_SCHEMA).
5. **Preserve misspellings** — The codebase uses `snash` (not "snapshot"), `constraines` (not "constraints"), and `Constraines`/`GetConstraines` throughout. **Do not "fix" these** — they are part of the API and JSON format.

## Key Structures

- `Config` — All connection and behavior settings (User, Pass, Host, Port, DB, FilesPath, File, Prefix, DTable, DColumn, DIndex, DConstraint, Optimize)
- `DBConn` — Wraps `*sql.DB`, holds Prefix and Scheme, provides `GetTables()`, `GetFields()`, `GetIndexes()`, `GetConstraines()`
- `Table`, `Field`, `Index`, `Constrain` — Database introspection structs (note: `Constrain` not `Constraint`)
- `Restore` — Has methods `Run()` and `runSQL()`

## Important Rules When Modifying Code

### JSON Format Stability
- The snapshot JSON format must remain backward-compatible. Adding new fields is safe; removing or renaming existing keys breaks existing snapshots.
- The `constraines` key must remain spelled as-is.
- Table names in the JSON are stored **without** the prefix.

### SQL Generation
- Table and column names must be backtick-quoted in generated SQL.
- The DEFAULT clause handling in `getDefault()` has special cases for `int`, `datetime`, and text/blob types — be careful when modifying.
- Constraint `DROP`/`ADD` operations happen in a phase where `FOREIGN_KEY_CHECKS=0` and `UNIQUE_CHECKS=0`.

### Dependencies
- Go 1.16 requires modules to be on (`GO111MODULE=on` is implicit).
- The `replace` directive in `go.mod` maps `github.com/serhioromano/mysqlsync/cmd` to `../cmd`. This is unusual and should be preserved unless the module structure changes.

### Testing
- There are currently **no tests**. When adding tests, consider that this tool interacts with a real MySQL database. Integration tests should use a disposable MySQL instance.
- For unit-testable logic, extract pure functions (e.g., SQL generation helpers, JSON diffing) from the DB-dependent code.

### Prefix Handling
- When generating SQL during restore, the prefix from `Config.Prefix` is added to table names.
- When comparing JSON data to current DB state, account for prefix differences between the snapshot source (`data["prefix"]`) and the restore target (`Config.Prefix`).

## Build & Run

```bash
# Install
go install

# Create snapshot with profile
mysqlsync snash -p=dev

# Restore with profile
mysqlsync restore -p=prod

# Cross-compile for Windows (via npm script)
npm run build
```

## Planned Features (Not Yet Implemented)
- Views synchronization
- Routines (stored procedures/functions) synchronization
- Triggers synchronization
- Package installation via yum, apt
