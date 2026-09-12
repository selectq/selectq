package core

import (
	"path/filepath"
	"testing"
)

func TestLinkExistingPurchaseInvoice(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "link.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = StoreInDB(db, AccountMeta{AccountNo: "link"}, []BankTransaction{
		{Date: "2026-09-12", WithdrawalAmt: 100, AccountHead: "Travel"},
		{Date: "2026-09-13", WithdrawalAmt: 100, AccountHead: "Travel"},
		{Date: "2026-09-14", WithdrawalAmt: 100, AccountHead: "Salary"},
	}, "test")
	if err != nil {
		t.Fatal(err)
	}
	p := PurchaseInvoice{InvoiceDate: "2026-09-12", PartyName: "Supplier", PartyAddress: "City", TotalAmount: 100, ExternalURL: "https://example.com/invoice", FileName: "invoice.pdf", FileType: "application/pdf", FileData: []byte("%PDF-1.4")}
	if err = SavePurchaseInvoice(db, &p); err != nil {
		t.Fatal(err)
	}
	if err = LinkPurchaseInvoice(db, p.ID, 3); err == nil {
		t.Fatal("salary accepted")
	}
	if err = LinkPurchaseInvoice(db, p.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err = LinkPurchaseInvoice(db, p.ID, 1); err != nil {
		t.Fatal("idempotent", err)
	}
	if err = LinkPurchaseInvoice(db, p.ID, 2); err == nil {
		t.Fatal("relinked invoice")
	}
	rows, err := GetPurchaseInvoices(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || *rows[0].BankTransactionID != 1 || rows[0].PartyName != p.PartyName || rows[0].ExternalURL != p.ExternalURL || rows[0].FileName != p.FileName {
		t.Fatal(rows)
	}
	var data []byte
	if err = db.QueryRow(`SELECT file_data FROM purchase_invoices WHERE id=?`, p.ID).Scan(&data); err != nil || string(data) != string(p.FileData) {
		t.Fatal("upload changed", err)
	}
	p.ID = 0
	p.BankTransactionID = nil
	if err = SavePurchaseInvoice(db, &p); err != nil {
		t.Fatal(err)
	}
	if err = LinkPurchaseInvoice(db, p.ID, 1); err == nil {
		t.Fatal("duplicate withdrawal link")
	}
}
