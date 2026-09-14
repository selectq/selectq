package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coregx/gxpdf"
)

func sampleSalesPDFInvoice() SalesInvoice {
	return SalesInvoice{InvoiceNumber: "031/2026-2027", InvoiceDate: "2026-07-01", DueDate: "2026-07-11", DueInDays: 10,
		BusinessPartnerName: "STUDIO SOUTH DESIGN", BillingAddress: "1115 BOULDERCREST DR NE\nATLANTA, GA 30316", SellerGSTIN: "06AKLPM9722C2Z3",
		ProjectName: "Canopy Houston", OurReference: "Canopy Houston", YourReference: "Canopy Houston", OrderNumber: "No P.O (project confirmed by email)",
		AdditionalInformation: "June services completed.", Currency: "USD", Amount: 1377,
		LineItems: []InvoiceLineItem{{Description: "Offshore Services for Public Area / Guestroom Package\nFor the month of June (153 hours)", Quantity: 153, Rate: 9, Amount: 1377}}}
}

func TestSalesPDF(t *testing.T) {
	inv := sampleSalesPDFInvoice()
	var annexure string
	for _, currency := range []string{"USD", "INR"} {
		for _, treatment := range []string{"intrastate", "interstate"} {
			inv.Currency, inv.GSTTreatment = currency, treatment
			inv.LineItems[0].GstPercent = 18
			data, err := SalesInvoicePDF(inv, CompanyProfile{CompanyName: "SELECT Q", GSTIN: "CHANGED"}, BusinessPartnerContact{Name: "Catherine/ Stacey"})
			if err != nil {
				t.Fatal(err)
			}
			doc, err := gxpdf.OpenFromBytes(data)
			if err != nil {
				t.Fatal(err)
			}
			if doc.PageCount() != 2 {
				t.Fatalf("expected two pages, got %d", doc.PageCount())
			}
			first, second := doc.Page(0).ExtractText(), doc.Page(1).ExtractText()
			for _, page := range doc.Pages() {
				for _, want := range []string{"Registered Office 189, Sector 31, Faridabad", "ritugandhi@selectq.in", "Registered Address", "ZIP 121003, INDIA"} {
					if !strings.Contains(page.ExtractText(), want) {
						t.Errorf("page %d missing letterhead text %q", page.Number(), want)
					}
				}
			}
			for _, want := range []string{"Canopy Houston", "No P.O", "Catherine/ Stacey", "1115 BOULDERCREST", "06AKLPM9722C2Z3", "June services completed", "transfer charges", "153 hours"} {
				if !strings.Contains(first, want) {
					t.Errorf("missing %q: %s", want, first)
				}
			}
			if strings.Contains(first, "CHANGED") {
				t.Error("current company GSTIN replaced saved GSTIN")
			}
			if currency == "USD" && (!strings.Contains(first, "Total: USD 1377.00") || strings.Contains(first, "CGST")) {
				t.Error(first)
			}
			if currency == "INR" && !strings.Contains(first, "Total: INR 1624.86") {
				t.Error(first)
			}
			if !strings.Contains(second, "50200020525472") || !strings.Contains(second, "HDFCINBBDEL") {
				t.Error(second)
			}
			if annexure != "" && annexure != second {
				t.Error("annexure changed across invoices")
			}
			annexure = second
			doc.Close()
			if output := os.Getenv("INVOICE_PDF_SAMPLE"); output != "" && currency == "USD" && treatment == "intrastate" {
				if err := os.WriteFile(output, data, 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	inv = sampleSalesPDFInvoice()
	inv.LineItems = nil
	for i := 0; i < 60; i++ {
		inv.LineItems = append(inv.LineItems, InvoiceLineItem{Description: fmt.Sprintf("Service row %02d", i), Quantity: 1, Rate: 9, Amount: 9})
	}
	inv.Amount = 540
	data, err := SalesInvoicePDF(inv, CompanyProfile{}, BusinessPartnerContact{})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := gxpdf.OpenFromBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	if doc.PageCount() < 3 {
		t.Fatal("overflow did not paginate")
	}
	for _, page := range doc.Pages() {
		if !strings.Contains(page.ExtractText(), "Registered Office 189") || !strings.Contains(page.ExtractText(), "ritugandhi@selectq.in") {
			t.Errorf("continuation page %d missing letterhead", page.Number())
		}
	}
	var all string
	for i := 0; i < doc.PageCount()-1; i++ {
		all += doc.Page(i).ExtractText()
	}
	for i := 0; i < 60; i++ {
		if !strings.Contains(all, fmt.Sprintf("Service row %02d", i)) {
			t.Errorf("lost row %d", i)
		}
	}
	if doc.Page(doc.PageCount()-1).ExtractText() != annexure {
		t.Error("overflow changed annexure")
	}
}

func TestInvoiceDetailsPersistence(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "invoice.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	partner, err := CreateBusinessPartner(db, BusinessPartner{Name: "Customer"})
	if err != nil {
		t.Fatal(err)
	}
	inv := sampleSalesPDFInvoice()
	inv.BusinessPartnerID = int(partner)
	id, err := CreateSalesInvoice(db, inv)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		got, err := GetSalesInvoiceByID(db, id)
		if err != nil {
			t.Fatal(err)
		}
		list, err := GetSalesInvoices(db)
		if err != nil {
			t.Fatal(err)
		}
		for _, saved := range []SalesInvoice{got, list[0]} {
			if saved.ProjectName != inv.ProjectName || saved.OurReference != inv.OurReference || saved.YourReference != inv.YourReference || saved.OrderNumber != inv.OrderNumber || saved.AdditionalInformation != inv.AdditionalInformation {
				t.Fatalf("lost details: %+v", saved)
			}
		}
		inv = got
		inv.ProjectName = "Revised project"
		inv.OurReference = "New ref"
		inv.YourReference = "Client ref"
		inv.OrderNumber = "PO 123"
		inv.AdditionalInformation = "Updated notes"
		if err := UpdateSalesInvoice(db, inv); err != nil {
			t.Fatal(err)
		}
	}
	if err := initInvoiceDetails(db); err != nil {
		t.Fatal(err)
	}
}
