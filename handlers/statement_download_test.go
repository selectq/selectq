package handlers

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/selectq/selectq/core"
	"github.com/xuri/excelize/v2"
)

func TestUploadAndDownloadStatement(t *testing.T) {
	db, err := core.InitDB(filepath.Join(t.TempDir(), "upload.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	srv := NewServer(db)
	f := excelize.NewFile()
	f.SetCellStr("Sheet1", "A1", "Account No :12345")
	row := []interface{}{"01/09/2026", "Receipt", "0001", "01/09/2026", 0, 100, 100}
	f.SetSheetRow("Sheet1", "A23", &row)
	workbook, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "original.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(workbook.Bytes()); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	r := httptest.NewRequest("POST", "/api/upload", &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	srv.UploadStatement(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var saved []byte
	if err := db.QueryRow(`SELECT workbook FROM statement_workbooks WHERE import_id=1`).Scan(&saved); err != nil || !bytes.Equal(saved, workbook.Bytes()) {
		t.Fatal("original upload was not retained", err)
	}
	for _, tc := range []struct {
		id     string
		status int
	}{{"1", 200}, {"999", 404}, {"bad", 400}, {"0", 400}} {
		r := mux.SetURLVars(httptest.NewRequest("GET", "/api/imports/"+tc.id+"/download", nil), map[string]string{"id": tc.id})
		w := httptest.NewRecorder()
		srv.DownloadStatement(w, r)
		if w.Code != tc.status {
			t.Fatal(tc.id, w.Code, w.Body.String())
		}
		if tc.status != 200 {
			continue
		}
		if !strings.Contains(w.Header().Get("Content-Disposition"), "original-classified.xlsx") {
			t.Fatal(w.Header())
		}
		out, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		value, err := out.GetCellValue("Sheet1", "J21")
		out.Close()
		if err != nil || value != "Invoice" {
			t.Fatal(value, err)
		}
	}
	if _, err := db.Exec(`DELETE FROM statement_workbooks`); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	srv.DownloadStatement(w, mux.SetURLVars(httptest.NewRequest("GET", "/api/imports/1/download", nil), map[string]string{"id": "1"}))
	if w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
}
