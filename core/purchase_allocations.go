package core

import (
	"database/sql"
	"fmt"
	"math"
)

type PurchaseAllocation struct {
	BankTransactionID int     `json:"bank_transaction_id"`
	PurchaseInvoiceID int     `json:"purchase_invoice_id"`
	Amount            float64 `json:"amount"`
}

func initPurchaseAllocations(tx *sql.Tx) error {
	_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS purchase_allocations (
 bank_transaction_id INTEGER NOT NULL REFERENCES bank_transactions(id),
 purchase_invoice_id INTEGER NOT NULL REFERENCES purchase_invoices(id),
 amount REAL NOT NULL CHECK(amount>0),
 PRIMARY KEY(bank_transaction_id,purchase_invoice_id));
 CREATE INDEX IF NOT EXISTS purchase_allocation_invoice ON purchase_allocations(purchase_invoice_id);
 INSERT OR IGNORE INTO purchase_allocations(bank_transaction_id,purchase_invoice_id,amount)
 SELECT p.bank_transaction_id,p.id,min(p.total_amount,b.withdrawal_amt)
 FROM purchase_invoices p JOIN bank_transactions b ON b.id=p.bank_transaction_id WHERE b.withdrawal_amt>0;`)
	return err
}

func AllocatePurchasePayment(db *sql.DB, id int, allocations []PurchaseAllocation) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = allocatePurchasePayment(tx, id, allocations); err != nil {
		return err
	}
	return tx.Commit()
}

// Older single-link clients allocate the available payment up to the invoice total.
func allocateLegacyPurchase(tx *sql.Tx, invoiceID, transactionID int) error {
	var amount float64
	if err := tx.QueryRow(`SELECT min(p.total_amount,b.withdrawal_amt) FROM purchase_invoices p,bank_transactions b WHERE p.id=? AND b.id=?`, invoiceID, transactionID).Scan(&amount); err != nil {
		return err
	}
	rows, err := tx.Query(`SELECT purchase_invoice_id,amount FROM purchase_allocations WHERE bank_transaction_id=? AND purchase_invoice_id<>?`, transactionID, invoiceID)
	if err != nil {
		return err
	}
	allocations := []PurchaseAllocation{{PurchaseInvoiceID: invoiceID, Amount: amount}}
	for rows.Next() {
		var a PurchaseAllocation
		if err = rows.Scan(&a.PurchaseInvoiceID, &a.Amount); err != nil {
			rows.Close()
			return err
		}
		allocations = append(allocations, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if err = allocatePurchasePayment(tx, transactionID, allocations); err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE purchase_invoices SET bank_transaction_id=? WHERE id=?`, transactionID, invoiceID)
	return err
}

func allocatePurchasePayment(tx *sql.Tx, id int, allocations []PurchaseAllocation) error {
	var withdrawal, deposit float64
	var head, sub, number string
	if err := tx.QueryRow(`SELECT withdrawal_amt,deposit_amt,COALESCE(account_head,''),COALESCE(sub_account_head,''),COALESCE(invoice_number,'') FROM bank_transactions WHERE id=?`, id).Scan(&withdrawal, &deposit, &head, &sub, &number); err != nil {
		return err
	}
	if !PurchaseEligible(withdrawal, head, sub, number) || deposit > 0 {
		return fmt.Errorf("This withdrawal is not eligible for purchase allocations")
	}
	var count int
	if err := tx.QueryRow(`SELECT count(*) FROM invoice_allocations WHERE bank_transaction_id=?`, id).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("This withdrawal already has sales allocations")
	}
	seen := map[int]bool{}
	total := 0.0
	for _, a := range allocations {
		if seen[a.PurchaseInvoiceID] || a.Amount <= 0 || math.IsNaN(a.Amount) || math.IsInf(a.Amount, 0) || math.Abs(a.Amount-money(a.Amount)) > 0.000001 {
			return fmt.Errorf("Allocations must be unique positive amounts with at most two decimal places")
		}
		seen[a.PurchaseInvoiceID] = true
		var amount, paid float64
		if err := tx.QueryRow(`SELECT total_amount,COALESCE((SELECT SUM(amount) FROM purchase_allocations WHERE purchase_invoice_id=p.id AND bank_transaction_id<>?),0) FROM purchase_invoices p WHERE id=?`, id, a.PurchaseInvoiceID).Scan(&amount, &paid); err != nil {
			return err
		}
		if money(paid+a.Amount) > money(amount) {
			return fmt.Errorf("Allocation exceeds outstanding amount for purchase invoice %d", a.PurchaseInvoiceID)
		}
		total += a.Amount
	}
	if money(total) > money(withdrawal) {
		return fmt.Errorf("Purchase allocations exceed the withdrawal amount")
	}
	if _, err := tx.Exec(`DELETE FROM purchase_allocations WHERE bank_transaction_id=?`, id); err != nil {
		return err
	}
	// The legacy column is retained for old clients, but no longer owns the links.
	if _, err := tx.Exec(`UPDATE purchase_invoices SET bank_transaction_id=NULL WHERE bank_transaction_id=?`, id); err != nil {
		return err
	}
	for _, a := range allocations {
		if _, err := tx.Exec(`INSERT INTO purchase_allocations VALUES(?,?,?)`, id, a.PurchaseInvoiceID, a.Amount); err != nil {
			return err
		}
	}
	return nil
}
