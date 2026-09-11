package core

import (
	"database/sql"
	"fmt"
	"math"
	"reflect"
	"strings"
)

type InvoiceAllocation struct {
	SalesInvoiceID int     `json:"sales_invoice_id"`
	Amount         float64 `json:"amount"`
}

func initAllocations(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(sales_invoices)`)
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid, nn, pk int
		var name, typ string
		var def interface{}
		if err := rows.Scan(&cid, &name, &typ, &nn, &def, &pk); err != nil {
			rows.Close()
			return err
		}
		if name == "is_closed" {
			found = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if !found {
		if _, err := db.Exec(`ALTER TABLE sales_invoices ADD COLUMN is_closed BOOLEAN NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS invoice_allocations (
 bank_transaction_id INTEGER NOT NULL REFERENCES bank_transactions(id),
 sales_invoice_id INTEGER NOT NULL REFERENCES sales_invoices(id),
 amount REAL NOT NULL CHECK(amount > 0),
 PRIMARY KEY(bank_transaction_id, sales_invoice_id));
 CREATE INDEX IF NOT EXISTS idx_allocation_invoice ON invoice_allocations(sales_invoice_id);`)
	return err
}
func money(v float64) float64 { return math.Round(v*100) / 100 }
func expectedReceipt(currency string, amount float64) float64 {
	if strings.EqualFold(currency, "INR") {
		return money(amount * .9)
	}
	return money(amount)
}
func invoiceBalance(db *sql.DB, inv *SalesInvoice) error {
	inv.ExpectedReceipt = expectedReceipt(inv.Currency, inv.Amount)
	if err := db.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM invoice_allocations WHERE sales_invoice_id=?`, inv.ID).Scan(&inv.ReceivedAmount); err != nil {
		return err
	}
	inv.ReceivedAmount = money(inv.ReceivedAmount)
	inv.OutstandingAmount = money(math.Max(0, inv.ExpectedReceipt-inv.ReceivedAmount))
	inv.IsSettled = inv.OutstandingAmount == 0
	return nil
}
func GetInvoiceAllocations(db *sql.DB, id int) ([]InvoiceAllocation, error) {
	rows, err := db.Query(`SELECT sales_invoice_id, amount FROM invoice_allocations WHERE bank_transaction_id=? ORDER BY sales_invoice_id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []InvoiceAllocation{}
	for rows.Next() {
		var a InvoiceAllocation
		if err := rows.Scan(&a.SalesInvoiceID, &a.Amount); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}
func sameInvoiceItems(a, b []InvoiceLineItem) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		a[i].ID = 0
		a[i].SalesInvoiceID = 0
		b[i].ID = 0
		b[i].SalesInvoiceID = 0
	}
	return reflect.DeepEqual(a, b)
}

// AllocateTransaction replaces a payment's allocations atomically. Amounts use the
// invoice currency; foreign receipts use the statement's recorded forex amount.
// A nil allocation list preserves existing links for older API clients.
func AllocateTransaction(db *sql.DB, id int, head, sub, legacy string, partner *int, allocations []InvoiceAllocation) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var deposit, withdrawal, forex float64
	var currency string
	if err = tx.QueryRow(`SELECT deposit_amt,withdrawal_amt,COALESCE(currency,''),forex_amount FROM bank_transactions WHERE id=?`, id).Scan(&deposit, &withdrawal, &currency, &forex); err != nil {
		return err
	}
	old := map[int]float64{}
	rows, err := tx.Query(`SELECT sales_invoice_id,amount FROM invoice_allocations WHERE bank_transaction_id=?`, id)
	if err != nil {
		return err
	}
	for rows.Next() {
		var invoice int
		var amount float64
		if err = rows.Scan(&invoice, &amount); err != nil {
			rows.Close()
			return err
		}
		old[invoice] = amount
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if allocations == nil {
		allocations = []InvoiceAllocation{}
		for invoice, amount := range old {
			allocations = append(allocations, InvoiceAllocation{invoice, amount})
		}
	}
	total := 0.0
	seen := map[int]bool{}
	numbers := []string{}
	if len(allocations) > 0 && (head != "Sales Invoice" || partner == nil || deposit <= 0 || withdrawal > 0) {
		return fmt.Errorf("invoice allocations require a sales receipt and business partner")
	}
	receipt := deposit
	for _, a := range allocations {
		if seen[a.SalesInvoiceID] || math.IsNaN(a.Amount) || math.IsInf(a.Amount, 0) || a.Amount <= 0 || math.Abs(a.Amount-money(a.Amount)) > .000001 {
			return fmt.Errorf("allocations must be unique positive amounts with at most two decimal places")
		}
		seen[a.SalesInvoiceID] = true
		var bp int
		var invCurrency, number string
		var amount, paid float64
		var closed bool
		if err = tx.QueryRow(`SELECT business_partner_id,currency,invoice_number,amount,is_closed FROM sales_invoices WHERE id=?`, a.SalesInvoiceID).Scan(&bp, &invCurrency, &number, &amount, &closed); err != nil {
			return fmt.Errorf("invoice %d: %w", a.SalesInvoiceID, err)
		}
		if bp != *partner {
			return fmt.Errorf("all invoices must belong to the selected business partner")
		}
		if closed && old[a.SalesInvoiceID] != a.Amount {
			return fmt.Errorf("closed invoice %s cannot receive new or changed allocations", number)
		}
		if strings.EqualFold(invCurrency, "INR") {
			if currency != "" && !strings.EqualFold(currency, "INR") {
				return fmt.Errorf("invoice currency must match receipt currency")
			}
		} else {
			if !strings.EqualFold(currency, invCurrency) || forex <= 0 {
				return fmt.Errorf("foreign invoices require a matching receipt currency and forex amount")
			}
			receipt = forex
		}
		if err = tx.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM invoice_allocations WHERE sales_invoice_id=? AND bank_transaction_id<>?`, a.SalesInvoiceID, id).Scan(&paid); err != nil {
			return err
		}
		if money(paid+a.Amount) > expectedReceipt(invCurrency, amount) {
			return fmt.Errorf("allocation exceeds outstanding amount for %s", number)
		}
		total += a.Amount
		numbers = append(numbers, number)
	}
	if money(total) > money(receipt) {
		return fmt.Errorf("allocations exceed the payment amount")
	}
	if _, err = tx.Exec(`DELETE FROM invoice_allocations WHERE bank_transaction_id=?`, id); err != nil {
		return err
	}
	for _, a := range allocations {
		if _, err = tx.Exec(`INSERT INTO invoice_allocations VALUES(?,?,?)`, id, a.SalesInvoiceID, a.Amount); err != nil {
			return err
		}
	}
	if len(numbers) > 0 {
		legacy = strings.Join(numbers, ", ")
	} else if len(old) > 0 {
		legacy = ""
	}
	if _, err = tx.Exec(`UPDATE bank_transactions SET account_head=?,sub_account_head=?,invoice_number=?,business_partner_id=? WHERE id=?`, head, sub, legacy, partner, id); err != nil {
		return err
	}
	return tx.Commit()
}
