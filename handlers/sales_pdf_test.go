package handlers

import (
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
