package core

import "database/sql"

func initInvoiceDetails(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, column := range []string{"project_name", "our_reference", "your_reference", "order_number", "additional_information"} {
		if _, err := addGSTColumn(tx, "sales_invoices", column, "TEXT NOT NULL DEFAULT ''"); err != nil {
			return err
		}
	}
	return tx.Commit()
}
