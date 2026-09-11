package core

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func browserTestDB(t *testing.T) (*sql.DB, *sql.Conn) {
	t.Helper()
	db, err := InitDB(filepath.Join(t.TempDir(), "browser.db"))
	if err != nil {
		t.Fatal(err)
	}
	conn, closeConn, err := BrowserConnection(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeConn(); db.Close() })
	return db, conn
}
func TestBrowserSchemaAndPagination(t *testing.T) {
	_, conn := browserTestDB(t)
	ctx := context.Background()
	_, err := conn.ExecContext(ctx, `CREATE TABLE "odd"";name" (id INTEGER PRIMARY KEY, value TEXT NOT NULL UNIQUE CHECK(length(value)>0), partner INTEGER REFERENCES business_partners(id)); INSERT INTO "odd"";name" VALUES(1,'one',NULL),(2,'two',NULL),(3,'three',NULL); CREATE VIEW browser_view AS SELECT id FROM "odd"";name";`)
	if err != nil {
		t.Fatal(err)
	}
	objects, err := BrowserObjects(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, o := range objects {
		found[o.Name] = true
	}
	for _, name := range []string{"sqlite_sequence", "invoice_sequences", "invoice_allocations", "browser_view", "odd\";name"} {
		if !found[name] {
			t.Errorf("missing %s", name)
		}
	}
	detail, err := BrowserDetail(ctx, conn, "odd\";name")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(detail.Object.SQL, "CHECK") || len(detail.Sections["Foreign keys"].Rows) != 1 || len(detail.Sections["Indexes and unique constraints"].Rows) != 1 {
		t.Fatalf("constraints missing: %+v", detail)
	}
	r, err := BrowserTableRows(ctx, conn, "odd\";name", 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Rows) != 2 || !r.Truncated {
		t.Fatalf("first page %+v", r)
	}
	r, err = BrowserTableRows(ctx, conn, "odd\";name", 2, 2)
	if err != nil || len(r.Rows) != 1 || r.Truncated {
		t.Fatalf("last page %+v %v", r, err)
	}
	if _, err = BrowserTableRows(ctx, conn, "accounts; DROP TABLE accounts", 100, 0); err == nil {
		t.Fatal("unknown identifier accepted")
	}
	for _, limit := range []int{0, 501} {
		if _, err = BrowserTableRows(ctx, conn, "accounts", limit, 0); err == nil {
			t.Fatal("bad limit accepted")
		}
	}
	settings, err := BrowserSettings(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	if settings["foreign_keys"].Rows[0][0] != int64(1) || len(settings["pragma_list"].Rows) == 0 {
		t.Fatalf("settings %+v", settings)
	}
}
func TestBrowserSQL(t *testing.T) {
	_, conn := browserTestDB(t)
	ctx := context.Background()
	run := func(query, mode string) DBResult {
		t.Helper()
		r, err := BrowserSQL(ctx, conn, query, mode)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	run(`CREATE TABLE browser_test (id INTEGER PRIMARY KEY, value TEXT);`, "execute")
	r := run(`INSERT INTO browser_test VALUES(1,'one'),(2,'two');`, "execute")
	if r.Changes != 2 {
		t.Fatalf("changes %+v", r)
	}
	r = run(`WITH vals AS (SELECT * FROM browser_test) SELECT id AS duplicate, value AS duplicate, NULL, x'00ff' FROM vals; -- end`, "query")
	if len(r.Rows) != 2 || r.Columns[0] != r.Columns[1] || r.Rows[0][2] != nil || r.Rows[0][3] != "0x00ff" || r.Changes != 0 {
		t.Fatalf("results %+v", r)
	}
	r = run(`UPDATE browser_test SET value='changed' RETURNING id;`, "query")
	if len(r.Rows) != 2 || r.Changes != 2 {
		t.Fatalf("returning %+v", r)
	}
	r = run(`WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<1002) SELECT x FROM n`, "query")
	if len(r.Rows) != 1000 || !r.Truncated {
		t.Fatal("query limit")
	}
	run(`CREATE TRIGGER browser_trigger AFTER INSERT ON browser_test BEGIN UPDATE browser_test SET value='trigger;' WHERE id=new.id; END; INSERT INTO browser_test VALUES(3,'three');`, "execute")
	r = run(`SELECT value FROM browser_test WHERE id=3`, "query")
	if r.Rows[0][0] != "trigger;" {
		t.Fatal("trigger script failed")
	}
	for _, q := range []string{"SELECT 1; DELETE FROM browser_test;", "-- comment only", "SELECT 1\x00; DELETE FROM browser_test"} {
		if _, err := BrowserSQL(ctx, conn, q, "query"); err == nil {
			t.Fatalf("invalid query accepted %q", q)
		}
	}
	if _, err := BrowserSQL(ctx, conn, "SELECT * FROM missing_table", "query"); err == nil {
		t.Fatal("SQL error missing")
	}
	if _, err := BrowserSQL(ctx, conn, "BEGIN; INSERT INTO browser_test VALUES(4,'pending');", "execute"); err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatal("uncommitted transaction not reported", err)
	}
	r = run(`SELECT COUNT(*) FROM browser_test WHERE id=4`, "query")
	if r.Rows[0][0] != int64(0) {
		t.Fatal("uncommitted insert persisted")
	}
	run(`BEGIN; INSERT INTO browser_test VALUES(5,'committed'); COMMIT;`, "execute")
	r = run(`SELECT COUNT(*) FROM browser_test WHERE id=5`, "query")
	if r.Rows[0][0] != int64(1) {
		t.Fatal("committed insert missing")
	}
}
func TestBrowserConnectionIsolation(t *testing.T) {
	db, _ := browserTestDB(t)
	ctx := context.Background()
	conn, closeConn, err := BrowserConnection(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = BrowserSQL(ctx, conn, "PRAGMA foreign_keys=OFF; PRAGMA query_only=ON;", "execute"); err != nil {
		t.Fatal(err)
	}
	closeConn()
	next, closeNext, err := BrowserConnection(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	defer closeNext()
	settings, err := BrowserSettings(ctx, next)
	if err != nil {
		t.Fatal(err)
	}
	if settings["query_only"].Rows[0][0] != int64(0) || settings["foreign_keys"].Rows[0][0] != int64(1) {
		t.Fatal("connection settings leaked")
	}
	cancelled, cancel := context.WithTimeout(ctx, time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)
	if _, err = BrowserSQL(cancelled, next, "SELECT 1", "query"); err == nil {
		t.Fatal("cancelled request executed")
	}
}
