package core

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestGSTSplit(t *testing.T) {
	for _, tc := range []struct {
		name, currency, seller, buyer, treatment string
		cgst, sgst, igst                         float64
	}{
		{"same state", "INR", "29AABCU9603R1ZP", "29ABCDE1234F1Z5", "intrastate", 9, 9, 0},
		{"different state", "INR", "29AABCU9603R1ZP", "27ABCDE1234F1Z5", "interstate", 0, 0, 18},
		{"missing buyer", "INR", "29AABCU9603R1ZP", "", "intrastate", 9, 9, 0},
		{"missing seller", "INR", "", "27ABCDE1234F1Z5", "intrastate", 9, 9, 0},
		{"both missing", "INR", "", "", "intrastate", 9, 9, 0},
		{"foreign", "USD", "29AABCU9603R1ZP", "27ABCDE1234F1Z5", "none", 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := InitDB(filepath.Join(t.TempDir(), "gst.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err = SaveCompanyProfile(db, CompanyProfile{GSTIN: tc.seller}); err != nil {
				t.Fatal(err)
			}
			bpID, err := CreateBusinessPartner(db, BusinessPartner{Name: "Tax customer", Addresses: []BPAddress{{Address: "Office", GSTIN: tc.buyer}}})
			if err != nil {
				t.Fatal(err)
			}
			addresses, err := GetBPAddresses(db, int(bpID))
			if err != nil {
				t.Fatal(err)
			}
			address := addresses[0].ID
			id, err := CreateSalesInvoice(db, SalesInvoice{BusinessPartnerID: int(bpID), AddressID: &address, InvoiceDate: "2026-09-11", Currency: tc.currency, GSTTreatment: "spoofed", SellerGSTIN: "99AAAAAAAAAAAAA", LineItems: []InvoiceLineItem{{Description: "Service", Amount: 209000, GstPercent: 18, CGSTPercent: 100, IGSTPercent: 100}}})
			if err != nil {
				t.Fatal(err)
			}
			inv, err := GetSalesInvoiceByID(db, id)
			if err != nil {
				t.Fatal(err)
			}
			li := inv.LineItems[0]
			if inv.GSTTreatment != tc.treatment || inv.SellerGSTIN != tc.seller || inv.BuyerGSTIN != tc.buyer || li.CGSTPercent != tc.cgst || li.SGSTPercent != tc.sgst || li.IGSTPercent != tc.igst {
				t.Fatalf("split %+v item %+v", inv, li)
			}
			if tc.currency == "INR" {
				if inv.GSTAmount != 37620 || inv.CGSTAmount+inv.SGSTAmount+inv.IGSTAmount != 37620 || inv.ExpectedReceipt != 225720 {
					t.Fatalf("totals %+v", inv)
				}
			} else if inv.GSTAmount != 0 || inv.ExpectedReceipt != 209000 {
				t.Fatalf("foreign tax %+v", inv)
			}
			list, err := GetSalesInvoices(db)
			if err != nil || len(list) != 1 || list[0].GSTTreatment != tc.treatment {
				t.Fatal("list split missing", err)
			}
		})
	}
}
func TestGSTINVersionAndInvoiceSnapshot(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "gst-history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = SaveCompanyProfile(db, CompanyProfile{GSTIN: "29AABCU9603R1ZP"}); err != nil {
		t.Fatal(err)
	}
	bpID, err := CreateBusinessPartner(db, BusinessPartner{Name: "History", Addresses: []BPAddress{{Address: "Office", GSTIN: " 27abcde1234f1z5 "}}})
	if err != nil {
		t.Fatal(err)
	}
	bp, err := GetBusinessPartnerByID(db, int(bpID))
	if err != nil {
		t.Fatal(err)
	}
	originalID := bp.Addresses[0].ID
	if bp.Addresses[0].GSTIN != "27ABCDE1234F1Z5" {
		t.Fatal("GSTIN not normalized")
	}
	invoice := SalesInvoice{BusinessPartnerID: bp.ID, AddressID: &originalID, InvoiceDate: "2026-09-11", Currency: "INR", LineItems: []InvoiceLineItem{{Description: "Service", Amount: 100, GstPercent: 18}}}
	id, err := CreateSalesInvoice(db, invoice)
	if err != nil {
		t.Fatal(err)
	}
	if err = SaveCompanyProfile(db, CompanyProfile{GSTIN: "27AABCU9603R1ZP"}); err != nil {
		t.Fatal(err)
	}
	stored, err := GetSalesInvoiceByID(db, id)
	if err != nil {
		t.Fatal(err)
	}
	if stored.GSTTreatment != "interstate" || stored.SellerGSTIN != "29AABCU9603R1ZP" {
		t.Fatal("profile edit changed saved invoice")
	}
	stored.GSTTreatment = "intrastate"
	stored.SellerGSTIN = "27AABCU9603R1ZP"
	if err = UpdateSalesInvoice(db, stored); err != nil {
		t.Fatal(err)
	}
	stored, err = GetSalesInvoiceByID(db, id)
	if err != nil || stored.GSTTreatment != "interstate" {
		t.Fatal("client overrode saved tax context", err)
	}
	newID, err := CreateSalesInvoice(db, invoice)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := GetSalesInvoiceByID(db, newID)
	if err != nil || fresh.GSTTreatment != "intrastate" {
		t.Fatal("new invoice did not use updated seller", err)
	}
	bp.Addresses[0].GSTIN = "29ABCDE1234F1Z5" // Same address text, different registration.
	if err = UpdateBusinessPartner(db, bp); err != nil {
		t.Fatal(err)
	}
	versions, err := GetBPAddresses(db, bp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 || !versions[0].IsArchived || versions[0].GSTIN != "27ABCDE1234F1Z5" || versions[1].PreviousAddressID == nil || *versions[1].PreviousAddressID != originalID {
		t.Fatalf("GSTIN version history %+v", versions)
	}
	stored, err = GetSalesInvoiceByID(db, id)
	if err != nil || stored.BuyerGSTIN != "27ABCDE1234F1Z5" || stored.GSTTreatment != "interstate" {
		t.Fatal("buyer history changed", err)
	}
	if _, err = db.Exec(`UPDATE bp_addresses SET gstin='29ABCDE1234F1Z5' WHERE id=?`, originalID); err == nil {
		t.Fatal("database allowed GSTIN overwrite")
	}
	if err = SaveCompanyProfile(db, CompanyProfile{GSTIN: "not a gstin"}); err == nil {
		t.Fatal("invalid seller GSTIN accepted")
	}
	if _, err = CreateBusinessPartner(db, BusinessPartner{Name: "Invalid", Addresses: []BPAddress{{Address: "Office", GSTIN: "invalid"}}}); err == nil {
		t.Fatal("invalid buyer GSTIN accepted")
	}
}
func TestGSTMigration(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "legacy-gst.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = createTables(db); err != nil {
		t.Fatal(err)
	}
	if err = initAllocations(db); err != nil {
		t.Fatal(err)
	}
	if err = initBPAddresses(db); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`UPDATE company_profile SET gstin='29AABCU9603R1ZP' WHERE id=1;
 INSERT INTO business_partners(name,tax_information) VALUES('Legacy','27ABCDE1234F1Z5');
 INSERT INTO sales_invoices(invoice_number,financial_year,business_partner_id,invoice_date,currency) VALUES('legacy','2026-2027',1,'2026-09-11','INR');`)
	if err != nil {
		t.Fatal(err)
	}
	if err = initGST(db); err != nil {
		t.Fatal(err)
	}
	inv, err := GetSalesInvoiceByID(db, 1)
	if err != nil {
		t.Fatal(err)
	}
	if inv.GSTTreatment != "intrastate" || inv.BuyerGSTIN != "" || inv.SellerGSTIN != "29AABCU9603R1ZP" {
		t.Fatalf("legacy migration %+v", inv)
	}
	if err = SaveCompanyProfile(db, CompanyProfile{GSTIN: "27AABCU9603R1ZP"}); err != nil {
		t.Fatal(err)
	}
	if err = initGST(db); err != nil {
		t.Fatal(err)
	}
	inv, err = GetSalesInvoiceByID(db, 1)
	if err != nil || inv.SellerGSTIN != "29AABCU9603R1ZP" {
		t.Fatal("repeat migration replaced snapshot", err)
	}
}
