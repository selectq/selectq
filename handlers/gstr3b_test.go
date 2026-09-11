package handlers

import (
	"bytes"
	"encoding/json"
	"github.com/selectq/selectq/core"
	"github.com/xuri/excelize/v2"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestGSTR3BSummaryAndDownload(t *testing.T) {
	db, err := core.InitDB(filepath.Join(t.TempDir(), "summary.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	statements := []string{
		`INSERT INTO business_partners(id,name,billing_address) VALUES(1,'Client','Current address')`,
		`INSERT INTO bp_addresses(id,business_partner_id,address,is_archived) VALUES(1,1,'Invoice address',0)`,
		`INSERT INTO reference_rates(rate_date,currency,units,rate_inr,source_file,source_sheet) VALUES('2026-03-31','USD',1,90,'',''),('2026-04-02','USD',1,92,'',''),('2026-04-02','JPY',100,60,'',''),('2026-04-05','EUR',1,100,'','')`,
		`INSERT INTO sales_invoices(invoice_number,financial_year,business_partner_id,address_id,invoice_date,currency,amount) VALUES('prior','2025-2026',1,1,'2026-03-31','INR',10),('fallback','2026-2027',1,1,'2026-04-01','USD',2),('exact','2026-2027',1,1,'2026-04-02','USD',2),('units','2026-2027',1,1,'2026-04-03','JPY',1000),('missing','2026-2027',1,1,'2026-04-03','EUR',2),('inr','2026-2027',1,1,'2027-03-31','INR',123.45),('next','2027-2028',1,1,'2027-04-01','INR',1)`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`UPDATE bp_addresses SET is_archived=1 WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	// INR totals include GST; foreign currency totals ignore line-item GST.
	if _, err := db.Exec(`INSERT INTO sales_invoice_line_items(sales_invoice_id,description,amount,gst_percent)
	 SELECT id,'Taxable item',amount,18 FROM sales_invoices WHERE invoice_number IN ('inr','exact')`); err != nil {
		t.Fatal(err)
	}
	s := NewServer(db)
	w := httptest.NewRecorder()
	s.GetGSTR3B(w, httptest.NewRequest("GET", "/api/gstr3b?financial_year=2026-2027", nil))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result core.GSTR3BSummary
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 5 || len(result.FinancialYears) != 3 {
		t.Fatalf("%+v", result)
	}
	expected := map[string]float64{"fallback": 180, "exact": 184, "units": 600, "inr": 145.67}
	for _, row := range result.Rows {
		if row.Address != "Invoice address" || row.InvoicedTo != "Client" {
			t.Fatalf("address %+v", row)
		}
		if row.InvoiceNumber == "missing" {
			if row.ExchangeRate != nil || row.ValueINR != nil || row.ReferenceDate != nil {
				t.Fatalf("future rate used %+v", row)
			}
			continue
		}
		if row.ValueINR == nil || *row.ValueINR != expected[row.InvoiceNumber] {
			t.Fatalf("conversion %+v", row)
		}
		if row.InvoiceNumber == "fallback" && (row.ReferenceDate == nil || *row.ReferenceDate != "2026-03-31") {
			t.Fatal("fallback date")
		}
		if row.InvoiceNumber == "inr" && (*row.ExchangeRate != 1 || row.ReferenceDate != nil) {
			t.Fatal("INR rate")
		}
		if row.InvoiceNumber == "inr" && row.Amount != 145.67 {
			t.Fatalf("INR amount must include GST: %v", row.Amount)
		}
	}
	w = httptest.NewRecorder()
	s.DownloadGSTR3B(w, httptest.NewRequest("GET", "/api/gstr3b/download?financial_year=2026-2027", nil))
	if w.Code != 200 || w.Header().Get("Content-Disposition") != `attachment; filename="GSTR3B-2026-2027.xlsx"` {
		t.Fatal(w.Code, w.Header())
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
	if len(rows) != 6 {
		t.Fatal(rows)
	}
	for _, row := range rows[1:] {
		if row[0] == "inr" && (row[2] != "145.67" || row[7] != "145.67") {
			t.Fatalf("Excel INR amounts must include GST: %v", row)
		}
		if row[0] == "missing" && (row[6] != "" || row[7] != "" || row[9] != "Missing reference rate") {
			t.Fatal(row)
		}
	}
	for _, fy := range []string{"", "2026-2026", "2026-27", "abcd-efgh"} {
		w = httptest.NewRecorder()
		s.DownloadGSTR3B(w, httptest.NewRequest("GET", "/?financial_year="+fy, nil))
		if w.Code != 400 {
			t.Fatal(fy, w.Code)
		}
	}
	w = httptest.NewRecorder()
	s.GetGSTR3B(w, httptest.NewRequest("GET", "/?financial_year=2020-2021", nil))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.Rows == nil || len(result.Rows) != 0 {
		t.Fatal("empty summary", w.Body.String(), err)
	}
}
