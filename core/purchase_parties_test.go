package core

import (
	"path/filepath"
	"testing"
)

func TestPurchasePartyCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "parties.db")
	db, err := InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	p := PurchaseInvoice{InvoiceDate: "2026-09-12", PartyName: "Supplier", PartyAddress: "City", PartyGSTIN: "27abcde1234f1z5", TotalAmount: 100}
	if err = SavePurchaseInvoice(db, &p); err != nil {
		t.Fatal(err)
	}
	party, err := GetPurchaseParty(db, "27ABCDE1234F1Z5")
	if err != nil || party.Name != "Supplier" {
		t.Fatal(party, err)
	}
	p.PartyAddress = "New address"
	if err = SavePurchaseInvoice(db, &p); err != nil {
		t.Fatal(err)
	}
	db.Close()
	db, err = InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	party, err = GetPurchaseParty(db, "27ABCDE1234F1Z5")
	if err != nil || party.Address != "New address" {
		t.Fatal(party, err)
	}
	p.PartyAddress = "Invalid save"
	p.TotalAmount = 0
	if err = SavePurchaseInvoice(db, &p); err == nil {
		t.Fatal("invalid saved")
	}
	party, err = GetPurchaseParty(db, "27ABCDE1234F1Z5")
	if err != nil || party.Address != "New address" {
		t.Fatal(party, err)
	}
	if _, err = db.Exec(`DELETE FROM purchase_party_cache`); err != nil {
		t.Fatal(err)
	}
	if err = initPurchaseParties(db); err != nil {
		t.Fatal(err)
	}
	party, err = GetPurchaseParty(db, "27ABCDE1234F1Z5")
	if err != nil || party.Address != "New address" {
		t.Fatal("backfill", party, err)
	}
}
