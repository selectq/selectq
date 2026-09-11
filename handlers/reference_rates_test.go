package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/selectq/selectq/core"
	"github.com/xuri/excelize/v2"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestReferenceRateManualAPI(t *testing.T) {
	db, err := core.InitDB(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewServer(db)
	call := func(method, date, currency, body string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "/api/reference-rates", strings.NewReader(body))
		r = mux.SetURLVars(r, map[string]string{"date": date, "currency": currency})
		w := httptest.NewRecorder()
		s.SaveReferenceRate(w, r)
		return w
	}
	body := `{"rate_date":"2026-08-31","currency":" jpy ","units":100,"rate_inr":59.72,"source_file":"spoof.xlsx"}`
	w := call("POST", "", "", body)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var saved core.ReferenceRate
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Currency != "JPY" || saved.Units != 100 || saved.SourceFile != "Manual entry" || saved.ImportedAt == "" {
		t.Fatalf("saved %+v", saved)
	}
	if w := call("POST", "", "", body); w.Code != 409 {
		t.Fatal("duplicate", w.Code, w.Body.String())
	}
	for _, bad := range []string{
		`{"rate_date":"2026-02-30","currency":"USD","units":1,"rate_inr":95}`,
		`{"rate_date":"2026-08-31","currency":"INR","units":1,"rate_inr":95}`,
		`{"rate_date":"2026-08-31","currency":"USD","units":0,"rate_inr":95}`,
		`{"rate_date":"2026-08-31","currency":"USD","units":1.5,"rate_inr":95}`,
		`{"rate_date":"2026-08-31","currency":"USD","units":1,"rate_inr":-1}`,
		body + ` {}`,
	} {
		if w := call("POST", "", "", bad); w.Code != 400 {
			t.Fatal("invalid", w.Code, w.Body.String())
		}
	}
	updated := `{"rate_date":"2026-08-28","currency":"JPY","units":100,"rate_inr":59.92}`
	if w := call("PUT", "2026-08-31", "JPY", updated); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call("PUT", "2026-08-31", "JPY", updated); w.Code != 404 {
		t.Fatal("missing", w.Code, w.Body.String())
	}
	if w := call("POST", "", "", body); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call("PUT", "2026-08-31", "JPY", updated); w.Code != 409 {
		t.Fatal("edit conflict", w.Code, w.Body.String())
	}
	list := httptest.NewRecorder()
	s.GetReferenceRates(list, httptest.NewRequest("GET", "/", nil))
	var rates []core.ReferenceRate
	if err := json.Unmarshal(list.Body.Bytes(), &rates); err != nil {
		t.Fatal(err)
	}
	if len(rates) != 2 || rates[0].RateDate != "2026-08-31" || rates[0].RateINR != 59.72 || rates[1].RateINR != 59.92 {
		t.Fatalf("list after conflict %+v", rates)
	}
}

func TestReferenceRateUploadAPI(t *testing.T) {
	db, err := core.InitDB(filepath.Join(t.TempDir(), "upload.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewServer(db)
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", "April Rates"); err != nil {
		t.Fatal(err)
	}
	for i, row := range [][]any{{"Date", "USD (INR / 1 USD)", "IDR (INR / 10000 IDR)"}, {"31/08/2026", 95.4509, 53.7816}, {"28/08/2026", 95.5614}} {
		if err := f.SetSheetRow("April Rates", fmt.Sprintf("A%d", i+1), &row); err != nil {
			t.Fatal(err)
		}
	}
	buffer, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	upload := func(filename, sheet string, data []byte) *httptest.ResponseRecorder {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatal(err)
		}
		if sheet != "" {
			if err := writer.WriteField("sheet", sheet); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodPost, "/api/reference-rates/import", &body)
		r.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		s.UploadReferenceRates(w, r)
		return w
	}
	for i := 0; i < 2; i++ {
		w := upload("ReferenceRate.xlsx", "", buffer.Bytes())
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var result core.ReferenceRateImport
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Inserted != 3*(1-i) || result.Skipped != 3*i || result.EmptySkipped != 1 || result.Sheet != "April Rates" {
			t.Fatalf("upload %d: %+v", i, result)
		}
	}
	for _, tc := range []struct {
		name, sheet string
		data        []byte
	}{
		{"rates.xls", "", buffer.Bytes()}, {"rates.xlsx", "missing", buffer.Bytes()}, {"rates.xlsx", "", []byte("invalid workbook")},
	} {
		w := upload(tc.name, tc.sheet, tc.data)
		if w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	rates, err := core.GetReferenceRates(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(rates) != 3 || rates[0].SourceFile != "ReferenceRate.xlsx" || rates[0].SourceSheet != "April Rates" {
		t.Fatalf("rates %+v", rates)
	}
}
