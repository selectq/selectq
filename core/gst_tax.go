package core

import (
	"database/sql"
	"fmt"
	"math"
	"regexp"
	"strings"
)

var gstinShape = regexp.MustCompile(`^[0-9]{2}[A-Z0-9]{13}$`)

func normalizeGSTIN(value string) (string, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value != "" && !gstinShape.MatchString(value) {
		return "", fmt.Errorf("GSTIN must be 15 alphanumeric characters beginning with a two-digit state code")
	}
	return value, nil
}
func gstTreatment(currency, seller, buyer string) string {
	if !strings.EqualFold(currency, "INR") {
		return "none"
	}
	if gstinShape.MatchString(seller) && gstinShape.MatchString(buyer) && seller[:2] != buyer[:2] {
		return "interstate"
	}
	return "intrastate"
}
func addGSTColumn(tx *sql.Tx, table, column, definition string) (bool, error) {
	rows, err := tx.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, err
	}
	found := false
	for rows.Next() {
		var cid, nn, pk int
		var name, typ string
		var def any
		if err = rows.Scan(&cid, &name, &typ, &nn, &def, &pk); err != nil {
			rows.Close()
			return false, err
		}
		if name == column {
			found = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return false, err
	}
	if found {
		return false, nil
	}
	_, err = tx.Exec("ALTER TABLE " + table + " ADD COLUMN " + column + " " + definition)
	return true, err
}
func initGST(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = addGSTColumn(tx, "bp_addresses", "gstin", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if _, err = addGSTColumn(tx, "sales_invoices", "seller_gstin", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	added, err := addGSTColumn(tx, "sales_invoices", "gst_treatment", "TEXT NOT NULL DEFAULT 'intrastate' CHECK(gst_treatment IN ('intrastate','interstate','none'))")
	if err != nil {
		return err
	}
	if added {
		// Preserve the information available at migration time; GSTIN is not inferred
		// from the legacy free-text business partner tax_information field.
		if _, err = tx.Exec(`UPDATE sales_invoices SET seller_gstin=UPPER(TRIM(COALESCE((SELECT gstin FROM company_profile WHERE id=1),'')));
   UPDATE sales_invoices SET gst_treatment=CASE WHEN UPPER(currency)<>'INR' THEN 'none'
   WHEN length(seller_gstin)=15 AND length(COALESCE((SELECT gstin FROM bp_addresses WHERE id=address_id),''))=15
    AND substr(seller_gstin,1,2)<>substr((SELECT gstin FROM bp_addresses WHERE id=address_id),1,2) THEN 'interstate' ELSE 'intrastate' END;`); err != nil {
			return err
		}
	}
	_, err = tx.Exec(`CREATE TRIGGER IF NOT EXISTS bp_address_gstin_immutable BEFORE UPDATE OF gstin ON bp_addresses
 WHEN NEW.gstin IS NOT OLD.gstin BEGIN SELECT RAISE(ABORT,'address GSTIN is immutable; create a new version'); END;`)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func invoiceGSTContext(tx *sql.Tx, inv *SalesInvoice, existing bool) error {
	var err error
	if existing {
		err = tx.QueryRow(`SELECT seller_gstin FROM sales_invoices WHERE id=?`, inv.ID).Scan(&inv.SellerGSTIN)
	} else {
		err = tx.QueryRow(`SELECT COALESCE(gstin,'') FROM company_profile WHERE id=1`).Scan(&inv.SellerGSTIN)
	}
	if err != nil {
		return err
	}
	if inv.SellerGSTIN, err = normalizeGSTIN(inv.SellerGSTIN); err != nil {
		return fmt.Errorf("seller %w", err)
	}
	inv.BuyerGSTIN = ""
	if inv.AddressID != nil {
		if err = tx.QueryRow(`SELECT gstin FROM bp_addresses WHERE id=? AND business_partner_id=?`, *inv.AddressID, inv.BusinessPartnerID).Scan(&inv.BuyerGSTIN); err != nil {
			return err
		}
	}
	inv.GSTTreatment = gstTreatment(inv.Currency, inv.SellerGSTIN, inv.BuyerGSTIN)
	for _, li := range inv.LineItems {
		if math.IsNaN(li.GstPercent) || math.IsInf(li.GstPercent, 0) || li.GstPercent < 0 || li.GstPercent > 100 {
			return fmt.Errorf("GST percentage must be between 0 and 100")
		}
	}
	return nil
}
func populateInvoiceGST(inv *SalesInvoice) {
	var gst float64
	inv.CGSTAmount, inv.SGSTAmount, inv.IGSTAmount = 0, 0, 0
	for i := range inv.LineItems {
		li := &inv.LineItems[i]
		li.CGSTPercent, li.SGSTPercent, li.IGSTPercent = 0, 0, 0
		if !strings.EqualFold(inv.Currency, "INR") {
			continue
		}
		gst += li.Amount * li.GstPercent / 100
		if inv.GSTTreatment == "interstate" {
			li.IGSTPercent = li.GstPercent
		} else {
			li.CGSTPercent = li.GstPercent / 2
			li.SGSTPercent = li.GstPercent / 2
		}
	}
	inv.GSTAmount = money(gst)
	if inv.GSTTreatment == "interstate" {
		inv.IGSTAmount = inv.GSTAmount
	} else if strings.EqualFold(inv.Currency, "INR") {
		inv.CGSTAmount = money(inv.GSTAmount / 2)
		inv.SGSTAmount = money(inv.GSTAmount - inv.CGSTAmount)
	}
}
