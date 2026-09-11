package core

import (
	"database/sql"
	"fmt"
	"strings"
)

type BPAddress struct {
	ID                int    `json:"id"`
	BusinessPartnerID int    `json:"business_partner_id"`
	Address           string `json:"address"`
	PreviousAddressID *int   `json:"previous_address_id"`
	IsArchived        bool   `json:"is_archived"`
	CreatedAt         string `json:"created_at"`
}

// Address text and ownership are immutable. A correction inserts a new row.
func initBPAddresses(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS bp_addresses (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 business_partner_id INTEGER NOT NULL REFERENCES business_partners(id) ON DELETE CASCADE,
 address TEXT NOT NULL CHECK(length(trim(address))>0),
 previous_address_id INTEGER REFERENCES bp_addresses(id),
 is_archived BOOLEAN NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
 );
 CREATE INDEX IF NOT EXISTS idx_bp_addresses_partner ON bp_addresses(business_partner_id, is_archived);
 CREATE UNIQUE INDEX IF NOT EXISTS idx_bp_address_version ON bp_addresses(previous_address_id) WHERE previous_address_id IS NOT NULL;
 CREATE TRIGGER IF NOT EXISTS bp_address_immutable BEFORE UPDATE OF id,business_partner_id,address,previous_address_id,created_at ON bp_addresses
 WHEN NEW.id IS NOT OLD.id OR NEW.business_partner_id IS NOT OLD.business_partner_id OR NEW.address IS NOT OLD.address OR NEW.previous_address_id IS NOT OLD.previous_address_id OR NEW.created_at IS NOT OLD.created_at
 BEGIN SELECT RAISE(ABORT,'address versions are immutable; create a new version'); END;
 CREATE TRIGGER IF NOT EXISTS bp_address_parent BEFORE INSERT ON bp_addresses
 WHEN NEW.previous_address_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM bp_addresses WHERE id=NEW.previous_address_id AND business_partner_id=NEW.business_partner_id)
 BEGIN SELECT RAISE(ABORT,'previous address must belong to the same partner'); END;`)
	if err != nil {
		return err
	}
	rows, err := tx.Query(`PRAGMA table_info(sales_invoices)`)
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid, nn, pk int
		var name, typ string
		var def any
		if err = rows.Scan(&cid, &name, &typ, &nn, &def, &pk); err != nil {
			rows.Close()
			return err
		}
		if name == "address_id" {
			found = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if !found {
		if _, err = tx.Exec(`ALTER TABLE sales_invoices ADD COLUMN address_id INTEGER REFERENCES bp_addresses(id);
   INSERT INTO bp_addresses(business_partner_id,address) SELECT id,billing_address FROM business_partners WHERE trim(COALESCE(billing_address,''))<>'' AND NOT EXISTS(SELECT 1 FROM bp_addresses WHERE business_partner_id=business_partners.id);
   UPDATE sales_invoices SET address_id=(SELECT MIN(id) FROM bp_addresses WHERE business_partner_id=sales_invoices.business_partner_id);`); err != nil {
			return err
		}
	}
	_, err = tx.Exec(`CREATE TRIGGER IF NOT EXISTS invoice_address_insert BEFORE INSERT ON sales_invoices
 WHEN NEW.address_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM bp_addresses WHERE id=NEW.address_id AND business_partner_id=NEW.business_partner_id AND is_archived=0)
 BEGIN SELECT RAISE(ABORT,'invoice requires an active address of the same partner'); END;
 CREATE TRIGGER IF NOT EXISTS invoice_address_update BEFORE UPDATE OF address_id,business_partner_id ON sales_invoices
 WHEN NEW.address_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM bp_addresses WHERE id=NEW.address_id AND business_partner_id=NEW.business_partner_id AND (is_archived=0 OR (NEW.address_id IS OLD.address_id AND NEW.business_partner_id=OLD.business_partner_id)))
 BEGIN SELECT RAISE(ABORT,'invoice requires an active address of the same partner'); END;`)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func GetBPAddresses(db *sql.DB, bpID int) ([]BPAddress, error) {
	rows, err := db.Query(`SELECT id,business_partner_id,address,previous_address_id,is_archived,created_at FROM bp_addresses WHERE business_partner_id=? ORDER BY id`, bpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	addresses := []BPAddress{}
	for rows.Next() {
		var a BPAddress
		if err = rows.Scan(&a.ID, &a.BusinessPartnerID, &a.Address, &a.PreviousAddressID, &a.IsArchived, &a.CreatedAt); err != nil {
			return nil, err
		}
		addresses = append(addresses, a)
	}
	return addresses, rows.Err()
}
func saveBPAddresses(tx *sql.Tx, bpID int, addresses []BPAddress, legacy string) error {
	// Legacy clients can still supply a single address. Empty/omitted legacy text
	// never removes history. Modern clients send addresses (omission preserves).
	if addresses == nil && strings.TrimSpace(legacy) != "" {
		var id int
		var current string
		err := tx.QueryRow(`SELECT id,address FROM bp_addresses WHERE business_partner_id=? AND is_archived=0 ORDER BY id LIMIT 1`, bpID).Scan(&id, &current)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if current != strings.TrimSpace(legacy) {
			addresses = []BPAddress{{ID: id, Address: legacy}}
		}
	}
	seen := map[int]bool{}
	for _, a := range addresses {
		text := strings.TrimSpace(a.Address)
		if text == "" {
			return fmt.Errorf("address cannot be blank")
		}
		if a.ID == 0 {
			if a.PreviousAddressID != nil {
				return fmt.Errorf("edit an existing address by its id to create a version")
			}
			if _, err := tx.Exec(`INSERT INTO bp_addresses(business_partner_id,address,is_archived) VALUES(?,?,?)`, bpID, text, a.IsArchived); err != nil {
				return err
			}
			continue
		}
		if seen[a.ID] {
			return fmt.Errorf("duplicate address id")
		}
		seen[a.ID] = true
		var original string
		var archived bool
		if err := tx.QueryRow(`SELECT address,is_archived FROM bp_addresses WHERE id=? AND business_partner_id=?`, a.ID, bpID).Scan(&original, &archived); err != nil {
			return fmt.Errorf("address does not belong to this partner")
		}
		if original != text {
			if archived {
				return fmt.Errorf("archived addresses cannot be edited; add a new address")
			}
			if _, err := tx.Exec(`UPDATE bp_addresses SET is_archived=1 WHERE id=?`, a.ID); err != nil {
				return err
			}
			if _, err := tx.Exec(`INSERT INTO bp_addresses(business_partner_id,address,previous_address_id,is_archived) VALUES(?,?,?,?)`, bpID, text, a.ID, a.IsArchived); err != nil {
				return err
			}
		} else {
			if archived && !a.IsArchived {
				return fmt.Errorf("archived addresses cannot be reactivated; add a new address")
			}
			if _, err := tx.Exec(`UPDATE bp_addresses SET is_archived=? WHERE id=?`, a.IsArchived, a.ID); err != nil {
				return err
			}
		}
	}
	// Retain the old scalar field as a compatibility display of the first active address.
	_, err := tx.Exec(`UPDATE business_partners SET billing_address=COALESCE((SELECT address FROM bp_addresses WHERE business_partner_id=? AND is_archived=0 ORDER BY id LIMIT 1),'') WHERE id=?`, bpID, bpID)
	return err
}
func validateInvoiceAddress(tx *sql.Tx, inv SalesInvoice, original *int, originalPartner int) error {
	if inv.AddressID == nil {
		// Retain legacy invoices without an address. New invoices for an addressless
		// partner are supported; configured partners must explicitly choose an address.
		if inv.ID != 0 && original == nil && originalPartner == inv.BusinessPartnerID {
			return nil
		}
		var count int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM bp_addresses WHERE business_partner_id=?`, inv.BusinessPartnerID).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("select an active billing address for this invoice")
		}
		return nil
	}
	var bp int
	var archived bool
	if err := tx.QueryRow(`SELECT business_partner_id,is_archived FROM bp_addresses WHERE id=?`, *inv.AddressID).Scan(&bp, &archived); err != nil {
		return fmt.Errorf("billing address does not exist")
	}
	if bp != inv.BusinessPartnerID {
		return fmt.Errorf("billing address must belong to the invoice partner")
	}
	unchanged := original != nil && *original == *inv.AddressID && originalPartner == bp
	if archived && !unchanged {
		return fmt.Errorf("archived billing addresses cannot be selected")
	}
	return nil
}
