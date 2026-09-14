package handlers

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/coregx/gxpdf"
	"github.com/gorilla/mux"
	"github.com/selectq/selectq/core"
)

func TestDownloadSalesInvoicePDF(t *testing.T) {
	db, err := core.InitDB(filepath.Join(t.TempDir(), "pdf.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	partner, err := core.CreateBusinessPartner(db, core.BusinessPartner{Name: "Customer", BillingAddress: "Original address"})
	if err != nil {
		t.Fatal(err)
	}
	bp, err := core.GetBusinessPartnerByID(db, int(partner))
	if err != nil {
		t.Fatal(err)
	}
	id, err := core.CreateSalesInvoice(db, core.SalesInvoice{BusinessPartnerID: int(partner), AddressID: &bp.Addresses[0].ID, InvoiceDate: "2026-09-13", Currency: "USD", ProjectName: "Saved project", LineItems: []core.InvoiceLineItem{{Description: "Work", Quantity: 1, Rate: 100, Amount: 100}}})
	if err != nil {
		t.Fatal(err)
	}
	// The original address version must remain the PDF's source.
	if _, err := db.Exec(`UPDATE bp_addresses SET is_archived=1 WHERE id=?; UPDATE business_partners SET billing_address='Changed address' WHERE id=?`, bp.Addresses[0].ID, partner); err != nil {
		t.Fatal(err)
	}
	srv := NewServer(db)
	t.Run("bulk download", func(t *testing.T) {
		secondPartner, err := core.CreateBusinessPartner(db, core.BusinessPartner{Name: "Second customer", BillingAddress: "Second address"})
		if err != nil {
			t.Fatal(err)
		}
		secondBP, err := core.GetBusinessPartnerByID(db, int(secondPartner))
		if err != nil {
			t.Fatal(err)
		}
		secondID, err := core.CreateSalesInvoice(db, core.SalesInvoice{BusinessPartnerID: int(secondPartner), AddressID: &secondBP.Addresses[0].ID, InvoiceDate: "2026-09-13", Currency: "USD", LineItems: []core.InvoiceLineItem{{Description: "Second invoice", Quantity: 1, Rate: 50, Amount: 50}}})
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			body   string
			status int
			count  int
		}{
			{fmt.Sprintf(`{"ids":[%d,%d,%d]}`, id, secondID, id), 200, 2},
			{`{"ids":[]}`, 400, 0},
			{`{"ids":[0]}`, 400, 0},
			{`{"ids":["bad"]}`, 400, 0},
			{`{"ids":[1]} {}`, 400, 0},
			{`{"ids":[` + strings.Repeat("1,", 100) + `1]}`, 400, 0},
			{fmt.Sprintf(`{"ids":[%d,999999]}`, id), 404, 0},
		} {
			w := httptest.NewRecorder()
			srv.DownloadSalesInvoicesZIP(w, httptest.NewRequest("POST", "/api/sales-invoices/pdf-download", strings.NewReader(tc.body)))
			if w.Code != tc.status {
				t.Fatalf("%s: %d %s", tc.body, w.Code, w.Body.String())
			}
			if tc.status != 200 {
				if strings.HasPrefix(w.Body.String(), "PK") {
					t.Fatal("partial ZIP returned")
				}
				continue
			}
			if w.Header().Get("Content-Type") != "application/zip" {
				t.Fatal(w.Header())
			}
			archive, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
			if err != nil {
				t.Fatal(err)
			}
			if len(archive.File) != tc.count {
				t.Fatalf("got %d entries", len(archive.File))
			}
			for i, file := range archive.File {
				if strings.ContainsAny(file.Name, `/\`) {
					t.Fatal(file.Name)
				}
				reader, err := file.Open()
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(reader)
				reader.Close()
				if err != nil {
					t.Fatal(err)
				}
				doc, err := gxpdf.OpenFromBytes(data)
				if err != nil {
					t.Fatal(err)
				}
				if doc.PageCount() != 2 || !strings.Contains(doc.Page(1).ExtractText(), "HDFCINBBDEL") {
					t.Fatal("missing invoice or annexure")
				}
				if i == 0 && !strings.Contains(doc.Page(0).ExtractText(), "Saved project") {
					t.Fatal("wrong first invoice")
				}
				if i == 1 && !strings.Contains(doc.Page(0).ExtractText(), "Second invoice") {
					t.Fatal("wrong second invoice")
				}
				doc.Close()
			}
		}
	})
	for _, tc := range []struct {
		id     string
		status int
	}{{strconv.Itoa(id), 200}, {"999999", 404}, {"bad", 400}} {
		r := mux.SetURLVars(httptest.NewRequest("GET", "/api/sales-invoices/"+tc.id+"/pdf", nil), map[string]string{"id": tc.id})
		w := httptest.NewRecorder()
		srv.DownloadSalesInvoicePDF(w, r)
		if w.Code != tc.status {
			t.Fatalf("%s: %d %s", tc.id, w.Code, w.Body.String())
		}
		if tc.status != 200 {
			continue
		}
		if w.Header().Get("Content-Type") != "application/pdf" || !strings.Contains(w.Header().Get("Content-Disposition"), "attachment;") || !strings.HasPrefix(w.Body.String(), "%PDF-") {
			t.Fatal(w.Header())
		}
		doc, err := gxpdf.OpenFromBytes(w.Body.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		text := doc.Page(0).ExtractText()
		doc.Close()
		if !strings.Contains(text, "Original address") || strings.Contains(text, "Changed address") || !strings.Contains(text, "Saved project") {
			t.Fatal(text)
		}
	}
}
