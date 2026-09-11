package core

import (
	"path/filepath"
	"testing"
)

func TestInvoiceAllocations(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "allocations.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	bp, err := CreateBusinessPartner(db, BusinessPartner{Name: "Customer", InvoiceCurrency: "INR"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := CreateBusinessPartner(db, BusinessPartner{Name: "Other", InvoiceCurrency: "INR"})
	if err != nil {
		t.Fatal(err)
	}
	partner := int(bp)
	create := func(p int) int {
		t.Helper()
		id, err := CreateSalesInvoice(db, SalesInvoice{BusinessPartnerID: p, InvoiceDate: "2026-09-11", Currency: "INR", LineItems: []InvoiceLineItem{{Description: "Service", Quantity: 1, Rate: 100, Amount: 100, GstPercent: 18}}})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	first, second, third := create(partner), create(partner), create(int(other))
	_, err = StoreInDB(db, AccountMeta{AccountNo: "test"}, []BankTransaction{{Date: "2026-09-11", DepositAmt: 150}, {Date: "2026-09-12", DepositAmt: 50}}, "test")
	if err != nil {
		t.Fatal(err)
	}
	save := func(id int, a []InvoiceAllocation) error {
		return AllocateTransaction(db, id, "Sales Invoice", "", "", &partner, a)
	}
	if err = save(1, []InvoiceAllocation{{first, 90}, {third, 60}}); err == nil {
		t.Fatal("mixed partners accepted")
	}
	if err = save(1, []InvoiceAllocation{{first, 90}, {second, 60}}); err != nil {
		t.Fatal(err)
	}
	inv, err := GetSalesInvoiceByID(db, first)
	if err != nil {
		t.Fatal(err)
	}
	if inv.ExpectedReceipt != 90 || inv.ReceivedAmount != 90 || !inv.IsSettled {
		t.Fatalf("unexpected INR balance: %+v", inv)
	}
	inv, err = GetSalesInvoiceByID(db, second)
	if err != nil {
		t.Fatal(err)
	}
	if inv.OutstandingAmount != 30 || inv.IsSettled {
		t.Fatalf("partial receipt: %+v", inv)
	}
	if err = save(2, []InvoiceAllocation{{second, 31}}); err == nil {
		t.Fatal("overpayment accepted")
	}
	if err = save(1, []InvoiceAllocation{{first, 90}, {second, 60}}); err != nil {
		t.Fatal("idempotent save:", err)
	}
	inv.IsClosed = true
	if err = UpdateSalesInvoice(db, inv); err != nil {
		t.Fatal(err)
	}
	if err = save(2, []InvoiceAllocation{{second, 1}}); err == nil {
		t.Fatal("closed invoice accepted new payment")
	}
	if err = save(1, []InvoiceAllocation{{first, 90}, {second, 60}}); err != nil {
		t.Fatal("closed existing link:", err)
	}
	inv.IsClosed = false
	if err = UpdateSalesInvoice(db, inv); err != nil {
		t.Fatal(err)
	}
	if err = save(2, []InvoiceAllocation{{second, 29.5}}); err != nil {
		t.Fatal(err)
	}
	inv, err = GetSalesInvoiceByID(db, second)
	if err != nil {
		t.Fatal(err)
	}
	if inv.IsSettled || inv.OutstandingAmount != .5 {
		t.Fatal("shortfall must remain open")
	}
	inv.BusinessPartnerID = int(other)
	if err = UpdateSalesInvoice(db, inv); err == nil {
		t.Fatal("linked invoice changed partner")
	}
	for _, a := range [][]InvoiceAllocation{{{first, 1}, {first, 1}}, {{first, -1}}, {{first, .001}}, {{third, 151}}} {
		if err = save(2, a); err == nil {
			t.Fatalf("invalid allocation accepted: %+v", a)
		}
	}
	txns, err := GetTransactions(db, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(txns[0].Allocations) != 2 {
		t.Fatal("links not returned")
	}
	if err = save(2, []InvoiceAllocation{}); err != nil {
		t.Fatal(err)
	}
	inv, err = GetSalesInvoiceByID(db, second)
	if err != nil || inv.OutstandingAmount != 30 {
		t.Fatalf("unlink balance: %+v %v", inv, err)
	}
	if err = initAllocations(db); err != nil {
		t.Fatal("repeat migration:", err)
	}
}

func TestForeignInvoiceAllocation(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "forex.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	bp, err := CreateBusinessPartner(db, BusinessPartner{Name: "Foreign customer", InvoiceCurrency: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	partner := int(bp)
	invoice, err := CreateSalesInvoice(db, SalesInvoice{BusinessPartnerID: partner, InvoiceDate: "2026-09-11", Currency: "USD", LineItems: []InvoiceLineItem{{Description: "Service", Quantity: 1, Rate: 100, Amount: 100}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = StoreInDB(db, AccountMeta{AccountNo: "forex"}, []BankTransaction{{Date: "2026-09-11", DepositAmt: 4200, Currency: "USD", ForexAmount: 50}, {Date: "2026-09-12", DepositAmt: 4200, Currency: "EUR", ForexAmount: 50}}, "test")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id     int
		amount float64
		fail   bool
	}{{1, 51, true}, {2, 50, true}, {1, 50, false}} {
		err = AllocateTransaction(db, tc.id, "Sales Invoice", "", "", &partner, []InvoiceAllocation{{invoice, tc.amount}})
		if (err != nil) != tc.fail {
			t.Fatalf("case %+v: %v", tc, err)
		}
	}
	inv, err := GetSalesInvoiceByID(db, invoice)
	if err != nil {
		t.Fatal(err)
	}
	if inv.ExpectedReceipt != 100 || inv.OutstandingAmount != 50 {
		t.Fatalf("foreign balance %+v", inv)
	}
}
