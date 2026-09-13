package core

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPurchaseInvoiceNumberUnique(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "unique.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	p := PurchaseInvoice{InvoiceNumber: " INV-001 ", InvoiceDate: "2026-09-12", PartyName: "Supplier", PartyAddress: "City", TotalAmount: 100}
	if err = SavePurchaseInvoice(db, &p); err != nil {
		t.Fatal(err)
	}
	first := p.ID
	p.ID = 0
	p.InvoiceNumber = "inv-001"
	p.PartyName = "Another supplier"
	if err = SavePurchaseInvoice(db, &p); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatal("duplicate accepted", err)
	}
	p.ID = first
	p.InvoiceNumber = "INV-001"
	if err = SavePurchaseInvoice(db, &p); err != nil {
		t.Fatal("unchanged own number", err)
	}
	p.ID = 0
	p.InvoiceNumber = "INV-002"
	if err = SavePurchaseInvoice(db, &p); err != nil {
		t.Fatal(err)
	}
	p.InvoiceNumber = "INV-001"
	if err = SavePurchaseInvoice(db, &p); err == nil {
		t.Fatal("duplicate edit")
	}
	if _, err = db.Exec(`UPDATE purchase_invoices SET invoice_number=' inv-001 ' WHERE id=?`, p.ID); err == nil {
		t.Fatal("direct SQL duplicate")
	}
	for i := 0; i < 2; i++ {
		p.ID = 0
		p.InvoiceNumber = ""
		if err = SavePurchaseInvoice(db, &p); err != nil {
			t.Fatal("optional number", err)
		}
	}
}
