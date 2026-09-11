package core

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
)

// DBResult retains column order and duplicate column names in arbitrary SQL results.
type DBResult struct {
	Columns   []string `json:"columns"`
	Rows      [][]any  `json:"rows"`
	Truncated bool     `json:"truncated"`
	Changes   int64    `json:"changes"`
}
type DBObject struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	TableName string `json:"table_name"`
	SQL       string `json:"sql"`
}
type DBObjectDetail struct {
	Object   DBObject            `json:"object"`
	Sections map[string]DBResult `json:"sections"`
}

// Every browser request owns a connection that is discarded afterwards. SQL
// PRAGMAs, ATTACH and transaction state must not leak into application requests.
func BrowserConnection(ctx context.Context, db *sql.DB) (*sql.Conn, func(), error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	closeConn := func() { _ = conn.Raw(func(any) error { return driver.ErrBadConn }); _ = conn.Close() }
	if _, err = conn.ExecContext(ctx, `PRAGMA foreign_keys=ON`); err != nil {
		closeConn()
		return nil, nil, err
	}
	return conn, closeConn, nil
}
func browserRows(ctx context.Context, conn *sql.Conn, query string, limit int, drain bool, args ...any) (DBResult, error) {
	result := DBResult{Columns: []string{}, Rows: [][]any{}}
	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	result.Columns, err = rows.Columns()
	if err != nil {
		return result, err
	}
	for rows.Next() {
		values := make([]any, len(result.Columns))
		ptrs := make([]any, len(values))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err = rows.Scan(ptrs...); err != nil {
			return result, err
		}
		if len(result.Rows) >= limit {
			result.Truncated = true
			if !drain {
				break
			}
			continue
		}
		for i, v := range values {
			switch value := v.(type) {
			case []byte:
				values[i] = "0x" + hex.EncodeToString(value)
			case float64:
				if math.IsInf(value, 0) || math.IsNaN(value) {
					values[i] = fmt.Sprint(value)
				}
			}
		}
		result.Rows = append(result.Rows, values)
	}
	return result, rows.Err()
}
func BrowserObjects(ctx context.Context, conn *sql.Conn) ([]DBObject, error) {
	rows, err := conn.QueryContext(ctx, `SELECT type,name,tbl_name,COALESCE(sql,'') FROM main.sqlite_schema ORDER BY type,name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	objects := []DBObject{}
	for rows.Next() {
		var o DBObject
		if err = rows.Scan(&o.Type, &o.Name, &o.TableName, &o.SQL); err != nil {
			return nil, err
		}
		objects = append(objects, o)
	}
	return objects, rows.Err()
}
func browserObject(ctx context.Context, conn *sql.Conn, name string) (DBObject, error) {
	var o DBObject
	err := conn.QueryRowContext(ctx, `SELECT type,name,tbl_name,COALESCE(sql,'') FROM main.sqlite_schema WHERE name=?`, name).Scan(&o.Type, &o.Name, &o.TableName, &o.SQL)
	return o, err
}
func BrowserDetail(ctx context.Context, conn *sql.Conn, name string) (DBObjectDetail, error) {
	detail := DBObjectDetail{Sections: map[string]DBResult{}}
	object, err := browserObject(ctx, conn, name)
	if err != nil {
		return detail, err
	}
	detail.Object = object
	queries := map[string]string{}
	if object.Type == "table" || object.Type == "view" {
		queries["Columns and primary key"] = `SELECT * FROM pragma_table_xinfo(?)`
		queries["Indexes and unique constraints"] = `SELECT * FROM pragma_index_list(?)`
		queries["Foreign keys"] = `SELECT * FROM pragma_foreign_key_list(?)`
	} else if object.Type == "index" {
		queries["Index columns"] = `SELECT * FROM pragma_index_xinfo(?)`
	}
	for title, query := range queries {
		r, err := browserRows(ctx, conn, query, 1000, false, name)
		if err != nil {
			return detail, err
		}
		detail.Sections[title] = r
	}
	return detail, nil
}
func BrowserTableRows(ctx context.Context, conn *sql.Conn, name string, limit, offset int) (DBResult, error) {
	if limit < 1 || limit > 500 || offset < 0 {
		return DBResult{}, fmt.Errorf("limit must be 1 to 500 and offset must be nonnegative")
	}
	o, err := browserObject(ctx, conn, name)
	if err != nil {
		return DBResult{}, err
	}
	if o.Type != "table" && o.Type != "view" {
		return DBResult{}, fmt.Errorf("select a table or view to browse rows")
	}
	// Validate against sqlite_schema and quote the identifier; names are not SQL.
	quoted := `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
	return browserRows(ctx, conn, `SELECT * FROM main.`+quoted+` LIMIT ? OFFSET ?`, limit, false, limit+1, offset)
}
func BrowserSettings(ctx context.Context, conn *sql.Conn) (map[string]DBResult, error) {
	result := map[string]DBResult{}
	for _, name := range []string{"database_list", "foreign_keys", "journal_mode", "synchronous", "busy_timeout", "cache_size", "page_size", "page_count", "freelist_count", "auto_vacuum", "encoding", "user_version", "schema_version", "application_id", "query_only", "recursive_triggers", "compile_options", "pragma_list"} {
		rows, err := browserRows(ctx, conn, "PRAGMA "+name, 1000, false)
		if err != nil {
			return nil, err
		}
		result[name] = rows
	}
	return result, nil
}

// Query mode accepts one statement. Quoted semicolons and SQL comments are
// skipped. Scripts (including trigger definitions) use Execute mode instead.
func singleBrowserStatement(query string) bool {
	ended := false
	token := false
	for i := 0; i < len(query); i++ {
		c := query[i]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			continue
		}
		if c == '-' && i+1 < len(query) && query[i+1] == '-' {
			for i < len(query) && query[i] != '\n' {
				i++
			}
			continue
		}
		if c == '/' && i+1 < len(query) && query[i+1] == '*' {
			i += 2
			for i+1 < len(query) && !(query[i] == '*' && query[i+1] == '/') {
				i++
			}
			i++
			continue
		}
		if ended {
			return false
		}
		if c == ';' {
			ended = true
			continue
		}
		token = true
		if c == '\'' || c == '"' || c == '`' || c == '[' {
			end := c
			if c == '[' {
				end = ']'
			}
			for i++; i < len(query); i++ {
				if query[i] == end {
					if end != ']' && i+1 < len(query) && query[i+1] == end {
						i++
						continue
					}
					break
				}
			}
		}
	}
	return token
}
func BrowserSQL(ctx context.Context, conn *sql.Conn, query, mode string) (DBResult, error) {
	result := DBResult{Columns: []string{}, Rows: [][]any{}}
	if strings.TrimSpace(query) == "" || strings.ContainsRune(query, 0) {
		return result, fmt.Errorf("enter SQL without NUL characters")
	}
	if mode != "query" && mode != "execute" {
		return result, fmt.Errorf("mode must be query or execute")
	}
	if mode == "query" && !singleBrowserStatement(query) {
		return result, fmt.Errorf("query mode accepts one statement; use execute for scripts")
	}
	var before int64
	if err := conn.QueryRowContext(ctx, `SELECT total_changes()`).Scan(&before); err != nil {
		return result, err
	}
	var err error
	if mode == "query" {
		result, err = browserRows(ctx, conn, query, 1000, true)
	} else {
		_, err = conn.ExecContext(ctx, query)
	}
	if err != nil {
		return result, err
	}
	var after int64
	if err = conn.QueryRowContext(ctx, `SELECT total_changes()`).Scan(&after); err != nil {
		return result, err
	}
	result.Changes = after - before
	// A request cannot leave a transaction open on its short-lived connection.
	if _, rollbackErr := conn.ExecContext(ctx, "ROLLBACK"); rollbackErr == nil {
		return result, fmt.Errorf("uncommitted transaction was rolled back; include COMMIT in the same script")
	}
	return result, nil
}
