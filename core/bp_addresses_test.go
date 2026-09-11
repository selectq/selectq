package core

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestBPAddressVersionHistory(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "addresses.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	id, err := CreateBusinessPartner(db, BusinessPartner{Name: "Address customer", Addresses: []BPAddress{{Address: "Office A"}, {Address: "Office B"}}, Contacts: []BusinessPartnerContact{{Name: "Accounts"}}})
	if err != nil {
		t.Fatal(err)
	}
	bp, err := GetBusinessPartnerByID(db, int(id))
	if err != nil {
		t.Fatal(err)
	}
	if len(bp.Addresses) != 2 || bp.Addresses[0].ID == bp.Addresses[1].ID {
		t.Fatalf("addresses %+v", bp.Addresses)
	}
	originalID := bp.Addresses[0].ID
	contactID := bp.Contacts[0].ID
	invoiceID, err := CreateSalesInvoice(db, SalesInvoice{BusinessPartnerID: bp.ID, AddressID: &originalID, ContactID: &contactID, InvoiceDate: "2026-09-11", Currency: "INR"})
	if err != nil {
		t.Fatal(err)
	}
	bp.Addresses[0].Address = "Office A, new location"
	if err = UpdateBusinessPartner(db, bp); err != nil {
		t.Fatal(err)
	}
	bp, err = GetBusinessPartnerByID(db, bp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(bp.Addresses) != 3 || bp.Addresses[0].Address != "Office A" || !bp.Addresses[0].IsArchived || bp.Addresses[2].IsArchived || bp.Addresses[2].PreviousAddressID == nil || *bp.Addresses[2].PreviousAddressID != originalID {
		t.Fatalf("version history %+v", bp.Addresses)
	}
	if bp.Contacts[0].ID != contactID {
		t.Fatal("address edit changed invoice contact id")
	}
	inv, err := GetSalesInvoiceByID(db, invoiceID)
	if err != nil {
		t.Fatal(err)
	}
	if inv.AddressID == nil || *inv.AddressID != originalID || inv.BillingAddress != "Office A" {
		t.Fatalf("invoice history changed %+v", inv)
	}
	if err = UpdateSalesInvoice(db, inv); err != nil {
		t.Fatal("existing archived address cannot be retained:", err)
	}
	if _, err = CreateSalesInvoice(db, SalesInvoice{BusinessPartnerID: bp.ID, AddressID: &originalID, InvoiceDate: "2026-09-11", Currency: "INR"}); err == nil {
		t.Fatal("archived address selected on new invoice")
	}
	currentID := bp.Addresses[2].ID
	freshID, err := CreateSalesInvoice(db, SalesInvoice{BusinessPartnerID: bp.ID, AddressID: &currentID, InvoiceDate: "2026-09-11", Currency: "INR"})
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := GetSalesInvoiceByID(db, freshID)
	if err != nil {
		t.Fatal(err)
	}
	fresh.AddressID = &originalID
	if err = UpdateSalesInvoice(db, fresh); err == nil {
		t.Fatal("another invoice selected archived address")
	}
	if _, err = CreateSalesInvoice(db, SalesInvoice{BusinessPartnerID: bp.ID, InvoiceDate: "2026-09-11", Currency: "INR"}); err == nil {
		t.Fatal("configured partner invoice accepted missing address")
	}
	bp.Addresses[1].IsArchived = true
	if err = UpdateBusinessPartner(db, bp); err != nil {
		t.Fatal(err)
	}
	again, err := GetBPAddresses(db, bp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 3 || !again[1].IsArchived {
		t.Fatalf("archive created unnecessary version %+v", again)
	}
	// Immutable history must also be protected when using raw SQL.
	if _, err = db.Exec(`UPDATE bp_addresses SET address='overwrite' WHERE id=?`, originalID); err == nil {
		t.Fatal("database allowed overwriting address history")
	}
	if _, err = db.Exec(`DELETE FROM bp_addresses WHERE id=?`, originalID); err == nil {
		t.Fatal("database allowed deleting invoice address")
	}
	all, err := GetSalesInvoices(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatal("invoice list")
	}
}

func TestBPAddressOwnershipAndAtomicity(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "ownership.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	create := func(name string) BusinessPartner {
		t.Helper()
		id, err := CreateBusinessPartner(db, BusinessPartner{Name: name, Addresses: []BPAddress{{Address: name + " location"}}})
		if err != nil {
			t.Fatal(err)
		}
		bp, err := GetBusinessPartnerByID(db, int(id))
		if err != nil {
			t.Fatal(err)
		}
		return bp
	}
	a, b := create("A"), create("B")
	foreignID := b.Addresses[0].ID
	if _, err = CreateSalesInvoice(db, SalesInvoice{BusinessPartnerID: a.ID, AddressID: &foreignID, InvoiceDate: "2026-09-11", Currency: "INR"}); err == nil {
		t.Fatal("cross-partner invoice address accepted")
	}
	a.Name = "Changed"
	a.Addresses = append(a.Addresses, BPAddress{ID: foreignID, Address: "Unauthorized"})
	if err = UpdateBusinessPartner(db, a); err == nil {
		t.Fatal("cross-partner address edit accepted")
	}
	saved, err := GetBusinessPartnerByID(db, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Name != "A" || len(saved.Addresses) != 1 {
		t.Fatal("failed address save partially updated partner")
	}
	for _, addresses := range [][]BPAddress{{{Address: " \n "}}, {{ID: saved.Addresses[0].ID, Address: "Revised"}, {ID: saved.Addresses[0].ID, Address: "Duplicate"}}} {
		saved.Addresses = addresses
		if err = UpdateBusinessPartner(db, saved); err == nil {
			t.Fatal("invalid address accepted")
		}
	}
	// Omitted address data preserves the address collection.
	saved.Addresses = nil
	saved.BillingAddress = ""
	if err = UpdateBusinessPartner(db, saved); err != nil {
		t.Fatal(err)
	}
	addresses, err := GetBPAddresses(db, a.ID)
	if err != nil || len(addresses) != 1 || addresses[0].Address != "A location" {
		t.Fatalf("omission changed addresses %+v %v", addresses, err)
	}
	// The database trigger independently rejects incorrect address ownership.
	_, err = db.Exec(`INSERT INTO sales_invoices(invoice_number,financial_year,business_partner_id,invoice_date,currency,address_id) VALUES('raw','2026-2027',?,'2026-09-11','INR',?)`, a.ID, foreignID)
	if err == nil {
		t.Fatal("database accepted cross-partner address")
	}
}

func TestBPAddressMigration(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = createTables(db); err != nil {
		t.Fatal(err)
	}
	if err = initAllocations(db); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO business_partners(name,billing_address) VALUES('Legacy customer','Historical office'),('No address','');
 INSERT INTO sales_invoices(invoice_number,financial_year,business_partner_id,invoice_date,currency) VALUES('001/2026-2027','2026-2027',1,'2026-09-11','INR'),('002/2026-2027','2026-2027',2,'2026-09-11','INR');`)
	if err != nil {
		t.Fatal(err)
	}
	if err = initBPAddresses(db); err != nil {
		t.Fatal(err)
	}
	var addressID int
	if err = db.QueryRow(`SELECT address_id FROM sales_invoices WHERE business_partner_id=1`).Scan(&addressID); err != nil {
		t.Fatal(err)
	}
	var noAddress *int
	if err = db.QueryRow(`SELECT address_id FROM sales_invoices WHERE business_partner_id=2`).Scan(&noAddress); err != nil || noAddress != nil {
		t.Fatal("empty address migrated incorrectly", err)
	}
	if _, err = db.Exec(`UPDATE bp_addresses SET is_archived=1 WHERE id=?`, addressID); err != nil {
		t.Fatal(err)
	}
	if err = initBPAddresses(db); err != nil {
		t.Fatal(err)
	}
	var count, retained int
	db.QueryRow(`SELECT COUNT(*) FROM bp_addresses`).Scan(&count)
	db.QueryRow(`SELECT address_id FROM sales_invoices WHERE business_partner_id=1`).Scan(&retained)
	if count != 1 || retained != addressID {
		t.Fatal("repeat migration rewrote history")
	}
	var ddl string
	if err = db.QueryRow(`SELECT sql FROM sqlite_schema WHERE name='sales_invoices'`).Scan(&ddl); err != nil || !strings.Contains(ddl, "REFERENCES bp_addresses") {
		t.Fatal("invoice address foreign key missing")
	}
}

func TestAddressForeignKeysOnPooledConnections(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "pooled-addresses.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	id, err := CreateBusinessPartner(db, BusinessPartner{Name: "Pooled partner", Addresses: []BPAddress{{Address: "Retained office"}}})
	if err != nil {
		t.Fatal(err)
	}
	addresses, err := GetBPAddresses(db, int(id))
	if err != nil {
		t.Fatal(err)
	}
	addressID := addresses[0].ID
	if _, err = CreateSalesInvoice(db, SalesInvoice{BusinessPartnerID: int(id), AddressID: &addressID, InvoiceDate: "2026-09-11", Currency: "INR"}); err != nil {
		t.Fatal(err)
	}
	first, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	var enabled int
	if err = second.QueryRowContext(context.Background(), "PRAGMA foreign_keys").Scan(&enabled); err != nil || enabled != 1 {
		t.Fatal("foreign keys not enabled on additional connection", err)
	}
	if _, err = second.ExecContext(context.Background(), "DELETE FROM bp_addresses WHERE id=?", addressID); err == nil {
		t.Fatal("pooled connection deleted an invoice address")
	}
}
