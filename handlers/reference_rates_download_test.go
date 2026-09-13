package handlers

import (
	"bytes"
	"github.com/selectq/selectq/core"
	"github.com/xuri/excelize/v2"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestReferenceRateDownloadRange(t *testing.T) {
	db, err := core.InitDB(filepath.Join(t.TempDir(), "export.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, r := range []core.ReferenceRate{{RateDate: "2026-09-01", Currency: "USD", Units: 1, RateINR: 90.1234}, {RateDate: "2026-09-02", Currency: "JPY", Units: 100, RateINR: 60.12}, {RateDate: "2026-09-03", Currency: "USD", Units: 1, RateINR: 91}} {
		if _, err = core.SaveReferenceRate(db, r, nil); err != nil {
			t.Fatal(err)
		}
	}
	s := NewServer(db)
	w := httptest.NewRecorder()
	s.DownloadReferenceRates(w, httptest.NewRequest("GET", "/?from=2026-09-01&to=2026-09-02", nil))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := f.GetRows("Sheet1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0][1] != "USD (INR / 1 USD)" || rows[0][2] != "JPY (INR / 100 JPY)" || rows[1][1] != "90.1234" || rows[2][1] != "" || rows[2][2] != "60.12" {
		t.Fatal(rows)
	}
	for _, tc := range []struct {
		query  string
		status int
	}{{"from=2026-09-03&to=2026-09-01", 400}, {"from=bad&to=2026-09-01", 400}, {"from=2020-01-01&to=2020-01-02", 404}, {"from=2026-09-01&to=2026-09-01", 200}} {
		w = httptest.NewRecorder()
		s.DownloadReferenceRates(w, httptest.NewRequest("GET", "/?"+tc.query, nil))
		if w.Code != tc.status {
			t.Fatal(tc, w.Code)
		}
	}
}
