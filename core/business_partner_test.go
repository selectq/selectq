package core

import (
	"path/filepath"
	"testing"
)

func TestBusinessPartnerContactsAfterUpdate(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "partners.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	p := BusinessPartner{Name: "Test partner", InvoiceCurrency: "USD"}
	id, err := CreateBusinessPartner(db, p)
	if err != nil {
		t.Fatal(err)
	}
	p.ID = int(id)
	p.Contacts = []BusinessPartnerContact{
		{Name: "Updated primary", Email: "primary@example.com", Phone: "123", IsPrimary: true},
		{Name: "Secondary", IsPrimary: false},
	}
	if err := UpdateBusinessPartner(db, p); err != nil {
		t.Fatal(err)
	}
	partners, err := GetBusinessPartners(db, "")
	if err != nil {
		t.Fatal(err)
	}
	detail, err := GetBusinessPartnerByID(db, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range append(partners, detail) {
		if len(got.Contacts) != 2 {
			t.Fatalf("expected 2 saved contacts, got %+v", got.Contacts)
		}
		for i, contact := range got.Contacts {
			want := p.Contacts[i]
			if contact.Name != want.Name || contact.Email != want.Email || contact.Phone != want.Phone || contact.IsPrimary != want.IsPrimary {
				t.Errorf("contact = %+v, want %+v", contact, want)
			}
		}
	}
}

func TestBusinessPartnerWithoutContacts(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "no-contacts.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, contacts := range [][]BusinessPartnerContact{nil, {}} {
		name := "Omitted contacts"
		if contacts != nil {
			name = "Empty contacts"
		}
		p := BusinessPartner{Name: name, InvoiceCurrency: "INR", Contacts: contacts}
		id, err := CreateBusinessPartner(db, p)
		if err != nil {
			t.Fatal(err)
		}
		p.ID = int(id)
		saved, err := GetBusinessPartnerByID(db, p.ID)
		if err != nil {
			t.Fatal(err)
		}
		if saved.Contacts == nil || len(saved.Contacts) != 0 {
			t.Fatalf("expected empty contact list: %+v", saved)
		}
		p.Contacts = []BusinessPartnerContact{{Name: "Optional contact", IsPrimary: true}}
		if err := UpdateBusinessPartner(db, p); err != nil {
			t.Fatal(err)
		}
		p.Contacts = contacts
		if err := UpdateBusinessPartner(db, p); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM business_partner_contacts WHERE business_partner_id=?`, p.ID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatal("removing last contact did not persist")
		}
	}
	// Prove the database itself allows a parent with no child contacts.
	if _, err := db.Exec(`INSERT INTO business_partners(name) VALUES('Direct SQL without contacts')`); err != nil {
		t.Fatal(err)
	}
}
