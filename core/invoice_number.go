package core

import (
	"database/sql"
	"fmt"
	"time"
)

func nextInvoiceNumber(tx *sql.Tx, fy string) (string, error) {
	// Acquire the write lock before allocating, and account for existing numbers.
	var sequence int
	err := tx.QueryRow(`INSERT INTO invoice_sequences (financial_year, last_number)
		SELECT ?, COALESCE(MAX(CAST(substr(invoice_number, 1, instr(invoice_number, '/') - 1) AS INTEGER)), 0) + 1
		FROM sales_invoices WHERE invoice_number GLOB '[0-9]*/' || ?
		ON CONFLICT(financial_year) DO UPDATE SET last_number = MAX(last_number + 1, excluded.last_number)
		RETURNING last_number`, fy, fy).Scan(&sequence)
	return fmt.Sprintf("%03d/%s", sequence, fy), err
}

// Repair invoices saved by older servers and reject future blank numbers.
func repairBlankInvoiceNumbers(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT id, invoice_date FROM sales_invoices WHERE trim(invoice_number) = '' ORDER BY invoice_date, id`)
	if err != nil {
		return err
	}
	var invoices []SalesInvoice
	for rows.Next() {
		var inv SalesInvoice
		if err := rows.Scan(&inv.ID, &inv.InvoiceDate); err != nil {
			rows.Close()
			return err
		}
		invoices = append(invoices, inv)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, inv := range invoices {
		fy, err := InvoiceFinancialYear(inv.InvoiceDate)
		if err != nil {
			return err
		}
		number, err := nextInvoiceNumber(tx, fy)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE sales_invoices SET invoice_number = ?, financial_year = ? WHERE id = ?`, number, fy, inv.ID); err != nil {
			return err
		}
	}
	_, err = tx.Exec(`
		CREATE TRIGGER IF NOT EXISTS sales_invoice_number_required_insert
		BEFORE INSERT ON sales_invoices WHEN NEW.invoice_number IS NULL OR trim(NEW.invoice_number) = ''
		BEGIN SELECT RAISE(ABORT, 'Invoice number must be assigned before saving'); END;
		CREATE TRIGGER IF NOT EXISTS sales_invoice_number_required_update
		BEFORE UPDATE OF invoice_number ON sales_invoices WHEN NEW.invoice_number IS NULL OR trim(NEW.invoice_number) = ''
		BEGIN SELECT RAISE(ABORT, 'Invoice number cannot be blank'); END;
	`)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// InvoiceFinancialYear uses the April through March financial year.
func InvoiceFinancialYear(date string) (string, error) {
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", fmt.Errorf("invalid invoice date: expected YYYY-MM-DD")
	}
	year := d.Year()
	if d.Month() < time.April {
		year--
	}
	return fmt.Sprintf("%04d-%04d", year, year+1), nil
}
