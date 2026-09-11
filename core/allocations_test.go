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
	_, err = StoreInDB(db, AccountMeta{AccountNo: "test"}, []BankTransaction{{Date: "2026-09-11", DepositAmt: 168}, {Date: "2026-09-12", DepositAmt: 50}}, "test")
	if err != nil {
		t.Fatal(err)
	}
	save := func(id int, a []InvoiceAllocation) error {
		return AllocateTransaction(db, id, "Sales Invoice", "", "", &partner, a)
	}
	if err = save(1, []InvoiceAllocation{{first, 108}, {third, 60}}); err == nil {
		t.Fatal("mixed partners accepted")
	}
	if err = save(1, []InvoiceAllocation{{first, 108}, {second, 60}}); err != nil {
		t.Fatal(err)
	}
	inv, err := GetSalesInvoiceByID(db, first)
	if err != nil {
		t.Fatal(err)
	}
	if inv.ExpectedReceipt != 108 || inv.ReceivedAmount != 108 || !inv.IsSettled {
		t.Fatalf("unexpected INR balance: %+v", inv)
	}
	inv, err = GetSalesInvoiceByID(db, second)
	if err != nil {
		t.Fatal(err)
	}
	if inv.OutstandingAmount != 48 || inv.IsSettled {
		t.Fatalf("partial receipt: %+v", inv)
	}
	if err = save(2, []InvoiceAllocation{{second, 49}}); err == nil {
		t.Fatal("overpayment accepted")
	}
	if err = save(1, []InvoiceAllocation{{first, 108}, {second, 60}}); err != nil {
		t.Fatal("idempotent save:", err)
	}
	inv.IsClosed = true
	if err = UpdateSalesInvoice(db, inv); err != nil {
		t.Fatal(err)
	}
	if err = save(2, []InvoiceAllocation{{second, 1}}); err == nil {
		t.Fatal("closed invoice accepted new payment")
	}
	if err = save(1, []InvoiceAllocation{{first, 108}, {second, 60}}); err != nil {
		t.Fatal("closed existing link:", err)
	}
	inv.IsClosed = false
	if err = UpdateSalesInvoice(db, inv); err != nil {
		t.Fatal(err)
	}
	if err = save(2, []InvoiceAllocation{{second, 47.5}}); err != nil {
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
	if err != nil || inv.OutstandingAmount != 48 {
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

func TestINRReceiptIncludesGSTAfterTDS(t *testing.T) {
	for _, tc := range []struct {
		name, currency                    string
		items                             []InvoiceLineItem
		expected, firstPayment, remaining float64
	}{
		{"clarified example", "INR", []InvoiceLineItem{{Description: "Services", Amount: 209000, GstPercent: 18}}, 225720, 188100, 37620},
		{"mixed GST rates", "INR", []InvoiceLineItem{{Description: "Services", Amount: 100, GstPercent: 18}, {Description: "Other", Amount: 200, GstPercent: 5}}, 298, 270, 28},
		{"zero GST", "INR", []InvoiceLineItem{{Description: "Services", Amount: 100, GstPercent: 0}}, 90, 40, 50},
		{"paise rounding", "INR", []InvoiceLineItem{{Description: "Services", Amount: 100.01, GstPercent: 18}}, 108.01, 90.01, 18},
		{"foreign currency unchanged", "USD", []InvoiceLineItem{{Description: "Services", Amount: 100, GstPercent: 18}}, 100, 40, 60},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := InitDB(filepath.Join(t.TempDir(), "gst-receipts.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			id, err := CreateBusinessPartner(db, BusinessPartner{Name: "Receipt customer", InvoiceCurrency: tc.currency})
			if err != nil {
				t.Fatal(err)
			}
			partner := int(id)
			invoiceID, err := CreateSalesInvoice(db, SalesInvoice{BusinessPartnerID: partner, InvoiceDate: "2026-09-11", Currency: tc.currency, LineItems: tc.items})
			if err != nil {
				t.Fatal(err)
			}
			before, err := GetSalesInvoiceByID(db, invoiceID)
			if err != nil {
				t.Fatal(err)
			}
			if before.ExpectedReceipt != tc.expected || before.OutstandingAmount != tc.expected {
				t.Fatalf("initial expected/outstanding: %+v", before)
			}
			_, err = StoreInDB(db, AccountMeta{AccountNo: "receipts"}, []BankTransaction{
				{Date: "2026-09-11", DepositAmt: tc.expected + 1, Currency: tc.currency, ForexAmount: tc.expected + 1},
				{Date: "2026-09-12", DepositAmt: tc.expected + 1, Currency: tc.currency, ForexAmount: tc.expected + 1},
			}, "receipts")
			if err != nil {
				t.Fatal(err)
			}
			save := func(transaction int, amount float64) error {
				return AllocateTransaction(db, transaction, "Sales Invoice", "", "", &partner, []InvoiceAllocation{{invoiceID, amount}})
			}
			if err = save(1, tc.firstPayment); err != nil {
				t.Fatal(err)
			}
			invoices, err := GetSalesInvoices(db)
			if err != nil {
				t.Fatal(err)
			}
			if len(invoices) != 1 || invoices[0].ExpectedReceipt != tc.expected || invoices[0].ReceivedAmount != tc.firstPayment || invoices[0].OutstandingAmount != tc.remaining || invoices[0].IsSettled {
				t.Fatalf("partial/list balance: %+v", invoices)
			}
			if err = save(2, tc.remaining+.01); err == nil {
				t.Fatal("allocation accepted one paisa above remaining")
			}
			links, err := GetInvoiceAllocations(db, 2)
			if err != nil || len(links) != 0 {
				t.Fatal("failed overpayment changed allocations", err)
			}
			if err = save(2, tc.remaining); err != nil {
				t.Fatal("GST-inclusive final payment rejected:", err)
			}
			settled, err := GetSalesInvoiceByID(db, invoiceID)
			if err != nil {
				t.Fatal(err)
			}
			if settled.ReceivedAmount != tc.expected || settled.OutstandingAmount != 0 || !settled.IsSettled {
				t.Fatalf("settled balance: %+v", settled)
			}
		})
	}
}
