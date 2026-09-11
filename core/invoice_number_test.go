package core

import (
	"path/filepath"
	"testing"
)

func TestInvoiceSequence(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "invoices.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	partner, err := CreateBusinessPartner(db, BusinessPartner{Name: "Customer"})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ date, number string }{
		{"2026-03-31", "001/2025-2026"},
		{"2026-04-01", "001/2026-2027"},
		{"2027-03-31", "002/2026-2027"},
		{"2027-04-01", "001/2027-2028"},
		{"2026-07-01", "003/2026-2027"},
	} {
		inv := SalesInvoice{InvoiceDate: c.date, BusinessPartnerID: int(partner), InvoiceNumber: "arbitrary", FinancialYear: "wrong", Currency: "INR", LineItems: []InvoiceLineItem{{Description: "Service", Quantity: 1, Rate: 100, Amount: 100}}}
		id, err := CreateSalesInvoice(db, inv)
		if err != nil {
			t.Fatal(err)
		}
		got, err := GetSalesInvoiceByID(db, id)
		if err != nil {
			t.Fatal(err)
		}
		if got.InvoiceNumber != c.number || got.FinancialYear != c.number[4:] || len(got.LineItems) != 1 {
			t.Fatalf("unexpected invoice: %+v", got)
		}
		got.InvoiceNumber = "changed"
		got.FinancialYear = "changed"
		if err := UpdateSalesInvoice(db, got); err != nil {
			t.Fatal(err)
		}
		updated, err := GetSalesInvoiceByID(db, id)
		if err != nil || updated.InvoiceNumber != c.number {
			t.Fatalf("number changed: %+v, %v", updated, err)
		}
		got.InvoiceDate = "2030-04-01"
		if err := UpdateSalesInvoice(db, got); err == nil {
			t.Fatal("accepted financial year change")
		}
	}
	if _, err := CreateSalesInvoice(db, SalesInvoice{InvoiceDate: "invalid"}); err == nil {
		t.Fatal("accepted invalid date")
	}
	// A failed insert must roll back the sequence allocation as well.
	if _, err := CreateSalesInvoice(db, SalesInvoice{InvoiceDate: "2026-08-01", BusinessPartnerID: -1}); err == nil {
		t.Fatal("accepted missing partner")
	}
	id, err := CreateSalesInvoice(db, SalesInvoice{InvoiceDate: "2026-08-01", BusinessPartnerID: int(partner), Currency: "INR"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := GetSalesInvoiceByID(db, id)
	if err != nil || got.InvoiceNumber != "004/2026-2027" {
		t.Fatalf("rollback failed: %+v, %v", got, err)
	}
}

func TestRepairBlankInvoiceNumber(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	partner, err := CreateBusinessPartner(db, BusinessPartner{Name: "Legacy customer"})
	if err != nil {
		t.Fatal(err)
	}
	// Reproduce a blank invoice written before the database safeguard existed.
	if _, err := db.Exec(`DROP TRIGGER sales_invoice_number_required_insert`); err != nil {
		t.Fatal(err)
	}
	result, err := db.Exec(`INSERT INTO sales_invoices (invoice_number, financial_year, business_partner_id, invoice_date, currency, amount) VALUES ('', '2026-2027', ?, '2026-09-10', 'USD', 100)`, partner)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := result.LastInsertId()
	db.Close()
	db, err = InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	got, err := GetSalesInvoiceByID(db, int(id))
	if err != nil || got.InvoiceNumber != "001/2026-2027" || got.Amount != 100 {
		t.Fatalf("repair: %+v, %v", got, err)
	}
	if _, err := db.Exec(`UPDATE sales_invoices SET invoice_number = '' WHERE id = ?`, id); err == nil {
		t.Fatal("accepted blank update")
	}
	if _, err := db.Exec(`INSERT INTO sales_invoices (invoice_number, financial_year, business_partner_id, invoice_date, currency, amount) VALUES (' ', '2026-2027', ?, '2026-09-10', 'USD', 100)`, partner); err == nil {
		t.Fatal("accepted blank insert")
	}
	if err := repairBlankInvoiceNumbers(db); err != nil {
		t.Fatal(err)
	}
	next, err := CreateSalesInvoice(db, SalesInvoice{InvoiceDate: "2026-09-11", BusinessPartnerID: int(partner), Currency: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	got, err = GetSalesInvoiceByID(db, next)
	if err != nil || got.InvoiceNumber != "002/2026-2027" {
		t.Fatalf("next: %+v, %v", got, err)
	}
}
