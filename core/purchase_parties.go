package core

import "database/sql"

type PurchaseParty struct {
	GSTIN   string `json:"gstin"`
	Name    string `json:"party_name"`
	Address string `json:"party_address"`
}

func initPurchaseParties(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS purchase_party_cache (
 gstin TEXT PRIMARY KEY, party_name TEXT NOT NULL, party_address TEXT NOT NULL);
 INSERT OR IGNORE INTO purchase_party_cache(gstin,party_name,party_address)
 SELECT upper(trim(p.party_gstin)),p.party_name,p.party_address FROM purchase_invoices p
 WHERE length(trim(p.party_gstin))=15 AND p.id=(SELECT max(q.id) FROM purchase_invoices q WHERE upper(trim(q.party_gstin))=upper(trim(p.party_gstin)));`)
	return err
}
func GetPurchaseParty(db *sql.DB, gstin string) (PurchaseParty, error) {
	var p PurchaseParty
	err := db.QueryRow(`SELECT gstin,party_name,party_address FROM purchase_party_cache WHERE gstin=?`, gstin).Scan(&p.GSTIN, &p.Name, &p.Address)
	return p, err
}
