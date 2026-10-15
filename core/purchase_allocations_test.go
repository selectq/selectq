package core

import (
	"path/filepath"
	"testing"
)

func TestPurchasePaymentAllocations(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "payments.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = StoreInDB(db, AccountMeta{AccountNo: "purchase"}, []BankTransaction{
		{Date: "2026-10-01", WithdrawalAmt: 4567}, {Date: "2026-10-02", WithdrawalAmt: 1000},
		{Date: "2026-10-03", DepositAmt: 4567}, {Date: "2026-10-04", WithdrawalAmt: 100, AccountHead: "Salary"},
	}, "test")
	if err != nil {
		t.Fatal(err)
	}
	invoices := []PurchaseInvoice{}
	for i, amount := range []float64{2000, 1500, 2067} {
		p := PurchaseInvoice{InvoiceDate: "2026-10-01", PartyName: "Supplier", PartyAddress: "City", TotalAmount: amount, FileName: "invoice.pdf", FileData: []byte("document")}
		if err = SavePurchaseInvoice(db, &p); err != nil {
			t.Fatalf("invoice %d: %v", i, err)
		}
		invoices = append(invoices, p)
	}
	allocations := []PurchaseAllocation{{PurchaseInvoiceID: invoices[0].ID, Amount: 2000}, {PurchaseInvoiceID: invoices[1].ID, Amount: 1500}, {PurchaseInvoiceID: invoices[2].ID, Amount: 1067}}
	if err = AllocatePurchasePayment(db, 1, allocations); err != nil {
		t.Fatal(err)
	}
	if err = AllocatePurchasePayment(db, 1, allocations); err != nil {
		t.Fatal("idempotent:", err)
	}
	rows, err := GetPurchaseInvoices(db)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range rows {
		if p.ID == invoices[2].ID && (p.PaidAmount != 1067 || p.OutstandingAmount != 1000 || len(p.Payments) != 1) {
			t.Fatalf("balance: %+v", p)
		}
	}
	bad := append([]PurchaseAllocation{}, allocations...)
	bad[2].Amount = 1067.01
	if err = AllocatePurchasePayment(db, 1, bad); err == nil {
		t.Fatal("withdrawal over-allocation accepted")
	}
	for _, a := range [][]PurchaseAllocation{
		{{PurchaseInvoiceID: invoices[0].ID, Amount: 0}},
		{{PurchaseInvoiceID: invoices[0].ID, Amount: -1}},
		{{PurchaseInvoiceID: invoices[0].ID, Amount: .001}},
		{{PurchaseInvoiceID: invoices[0].ID, Amount: 1}, {PurchaseInvoiceID: invoices[0].ID, Amount: 1}},
		{{PurchaseInvoiceID: 99999, Amount: 1}},
	} {
		if err = AllocatePurchasePayment(db, 1, a); err == nil {
			t.Fatalf("invalid accepted: %+v", a)
		}
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM purchase_allocations WHERE bank_transaction_id=1`).Scan(&count); err != nil || count != 3 {
		t.Fatal("failed save changed links", count, err)
	}
	if err = AllocatePurchasePayment(db, 2, []PurchaseAllocation{{PurchaseInvoiceID: invoices[0].ID, Amount: 1}}); err == nil {
		t.Fatal("settled invoice overpayment accepted")
	}
	if err = AllocatePurchasePayment(db, 2, []PurchaseAllocation{{PurchaseInvoiceID: invoices[2].ID, Amount: 1000}}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int{3, 4} {
		if err = AllocatePurchasePayment(db, id, []PurchaseAllocation{{PurchaseInvoiceID: invoices[2].ID, Amount: 1}}); err == nil {
			t.Fatal("ineligible transaction accepted")
		}
	}
	p := invoices[2]
	p.TotalAmount = 2000
	if err = SavePurchaseInvoice(db, &p); err == nil {
		t.Fatal("reduced invoice below payments")
	}
	if err = AllocateTransaction(db, 1, "Salary", "", "", nil, nil); err == nil {
		t.Fatal("invalid reclassification accepted")
	}
	if err = AllocatePurchasePayment(db, 1, []PurchaseAllocation{}); err != nil {
		t.Fatal(err)
	}
	if err = initPurchaseInvoices(db); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM purchase_allocations WHERE bank_transaction_id=1`).Scan(&count); err != nil || count != 0 {
		t.Fatal("removed links restored")
	}
	rows, err = GetPurchaseInvoices(db)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range rows {
		if p.ID == invoices[2].ID && (p.PaidAmount != 1000 || p.OutstandingAmount != 1067) {
			t.Fatalf("unlink balance: %+v", p)
		}
	}
}

func TestPurchaseAllocationMigration(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = StoreInDB(db, AccountMeta{AccountNo: "legacy"}, []BankTransaction{{Date: "2026-10-01", WithdrawalAmt: 100}}, "test")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO purchase_invoices(bank_transaction_id,invoice_date,party_name,party_address,total_amount,file_name,file_data) VALUES(1,'2026-10-01','Supplier','City',150,'test.pdf',?)`, []byte("original"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = initPurchaseInvoices(db); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := GetPurchaseInvoices(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].PaidAmount != 100 || rows[0].OutstandingAmount != 50 || len(rows[0].Payments) != 1 {
		t.Fatalf("migration: %+v", rows)
	}
	if err = AllocatePurchasePayment(db, 1, []PurchaseAllocation{}); err != nil {
		t.Fatal(err)
	}
	if err = initPurchaseInvoices(db); err != nil {
		t.Fatal(err)
	}
	var count int
	var data []byte
	if err = db.QueryRow(`SELECT count(*) FROM purchase_allocations`).Scan(&count); err != nil || count != 0 {
		t.Fatal("link resurrected")
	}
	if err = db.QueryRow(`SELECT file_data FROM purchase_invoices WHERE id=1`).Scan(&data); err != nil || string(data) != "original" {
		t.Fatal("document changed")
	}
}
