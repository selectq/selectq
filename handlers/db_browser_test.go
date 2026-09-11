package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/selectq/selectq/core"
)

func TestDBBrowserEndpoints(t *testing.T) {
	db, err := core.InitDB(filepath.Join(t.TempDir(), "browser-api.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := NewServer(db)
	cases := []struct {
		name              string
		handler           http.HandlerFunc
		method, url, body string
		status            int
		contains          string
	}{
		{"objects", server.BrowserObjects, "GET", "/api/db-browser/objects", "", 200, "invoice_allocations"},
		{"constraints", server.BrowserDetail, "GET", "/api/db-browser/detail?name=invoice_allocations", "", 200, "Foreign keys"},
		{"missing", server.BrowserDetail, "GET", "/api/db-browser/detail?name=missing", "", 404, ""},
		{"pagination", server.BrowserRows, "GET", "/api/db-browser/rows?name=accounts&limit=0", "", 400, "limit"},
		{"invalid offset", server.BrowserRows, "GET", "/api/db-browser/rows?name=accounts&offset=abc", "", 400, "offset"},
		{"query", server.BrowserSQL, "POST", "/api/db-browser/sql", `{"sql":"SELECT 42 AS answer","mode":"query"}`, 200, "answer"},
		{"invalid JSON", server.BrowserSQL, "POST", "/api/db-browser/sql", `{`, 400, "invalid SQL request"},
		{"multiple JSON", server.BrowserSQL, "POST", "/api/db-browser/sql", `{} {}`, 400, "one JSON"},
		{"SQL error", server.BrowserSQL, "POST", "/api/db-browser/sql", `{"sql":"SELECT * FROM missing","mode":"query"}`, 400, "missing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.url, bytes.NewBufferString(tc.body))
			w := httptest.NewRecorder()
			tc.handler(w, req)
			if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.contains) {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
}
