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
