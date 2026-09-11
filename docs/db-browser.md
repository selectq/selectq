# SQLite DB Browser

Open **DB Browser** from the sidebar. The browser works against the application's
current database, including internal SQLite tables. No database migration is
needed for this feature.

## Browse schema and data

The left panel filters objects by name and groups tables, views, indexes,
sequences, triggers and system settings. Select a table or view to browse rows
in pages of 100. The right panel provides **Rows** and **Structure & constraints**.

Structure shows the original SQLite definition, columns (including generated or
hidden columns), primary-key positions, NOT NULL flags, defaults, index metadata,
unique constraints and foreign keys. CHECK expressions are shown in the original
CREATE statement. Select an index to inspect its indexed columns and ordering.
SQLite-generated indexes may have no standalone SQL definition; their parent
table contains the corresponding primary-key or unique declaration.

SQLite has no standalone sequence objects. The Sequences group exposes
`sqlite_sequence`, which tracks AUTOINCREMENT tables, and this application's
`invoice_sequences` table. Both are also ordinary tables in the Tables group.

System settings show commonly used PRAGMAs, database files, compile options and
the complete available PRAGMA name list. Use the SQL editor for other PRAGMAs,
such as `PRAGMA table_info('sales_invoices')` or `PRAGMA foreign_key_check`.

## Execute SQL

The SQL editor offers two modes:

- **Statement with results:** one statement, with up to 1,000 displayed rows.
  Supports SELECT, CTEs, PRAGMAs and writes with RETURNING. Quoted semicolons and
  comments are supported. The statement is fully consumed even when displayed
  results are truncated, so RETURNING writes complete normally.
- **Execute script:** one or more statements, including schema changes and
  trigger definitions. Displays the number of row changes, including changes
  made by triggers; it does not display SELECT results. DDL usually reports zero
  row changes even when the schema changed.

Ctrl+Enter runs the editor. SQL remains available after errors. Objects and
settings refresh after execution, including failed scripts that may have already
committed earlier statements. Use Reload when revisiting table rows or structure.

SQL writes apply directly and bypass the invoice/classification validation used
by the normal application screens. Query mode is a result-display mode, not a
read-only restriction. The feature uses the same access model as the existing
application; it does not add separate database-user authentication.

Each request owns a SQLite connection that is discarded afterwards. Temporary
objects, attached databases and connection-specific PRAGMAs are scoped to that
request. Persistent database settings and committed writes remain. Browser
connections start with foreign-key enforcement enabled.

For an atomic batch, include transaction control in the same Execute script:

```sql
BEGIN;
UPDATE company_profile SET company_name = 'Example Company' WHERE id = 1;
COMMIT;
```

An unfinished transaction is rolled back and reported as an error. Without an
explicit transaction, earlier committed statements remain when a later statement
fails. BEGIN in one request and COMMIT in another is not supported.

Requests time out after 10 seconds. SQL request bodies are limited to 1 MiB.
NULL is displayed explicitly and BLOB values use `0x`-prefixed hexadecimal.
Duplicate SQL result column names are preserved. Table pagination reflects the
current data and does not hold a snapshot across requests; use an explicit
ORDER BY in the editor when a particular row ordering matters.

## API

| Method | Endpoint | Purpose |
| --- | --- | --- |
| GET | `/api/db-browser/objects` | Main database schema objects, including internal tables and indexes. |
| GET | `/api/db-browser/detail?name=...` | Definition and metadata sections for an object. |
| GET | `/api/db-browser/rows?name=...&limit=100&offset=0` | Table/view data; limit accepts 1 to 500. |
| GET | `/api/db-browser/settings` | Read current settings and PRAGMA inventory. |
| POST | `/api/db-browser/sql` | Execute `{ "sql": "SELECT 1", "mode": "query" }` or an `execute` script. |

Row results contain ordered `columns`, a `rows` array, `truncated`, and `changes`.
Metadata lookup binds the object name. Data browsing validates names against
`sqlite_schema` and quotes identifiers before constructing its SELECT.

The implementation is in [core/db_browser.go](../core/db_browser.go),
[handlers/db_browser.go](../handlers/db_browser.go), and the
[DB Browser component](../frontend/src/app/db-browser/db-browser.component.ts).

## Verification

Backend coverage includes schema metadata, quoted identifiers, pagination,
SQL writes and RETURNING, trigger scripts, connection isolation, unfinished
transaction rollback, and API errors. UI tests cover rendered rows, structure,
paging, cancelled stale requests and SQL errors.

```powershell
go test ./core ./handlers .
cd frontend
npm.cmd run build -- --configuration development
npm.cmd test -- --watch=false --karma-config=karma.browser-check.cjs --include=src/app/db-browser/db-browser.component.spec.ts
```

The browser test configuration runs Chrome headlessly with GPU disabled for
compatibility with this workspace.
