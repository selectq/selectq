package core

import (
	"database/sql"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"
)

type PurchaseInvoice struct {
	ExternalURL       string  `json:"external_url"`
	ID                int     `json:"id"`
	BankTransactionID *int    `json:"bank_transaction_id"`
	InvoiceNumber     string  `json:"invoice_number"`
	InvoiceDate       string  `json:"invoice_date"`
	PartyName         string  `json:"party_name"`
	PartyAddress      string  `json:"party_address"`
	PartyGSTIN        string  `json:"party_gstin"`
	TotalAmount       float64 `json:"total_amount"`
	FileName          string  `json:"file_name"`
	FileType          string  `json:"-"`
	FileData          []byte  `json:"-"`
}

func initPurchaseInvoices(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS purchase_invoices (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 bank_transaction_id INTEGER UNIQUE REFERENCES bank_transactions(id),
 invoice_number TEXT NOT NULL DEFAULT '', invoice_date TEXT NOT NULL,
 party_name TEXT NOT NULL, party_address TEXT NOT NULL, party_gstin TEXT NOT NULL DEFAULT '',
 total_amount REAL NOT NULL CHECK(total_amount>0),
 file_name TEXT NOT NULL DEFAULT '', file_type TEXT NOT NULL DEFAULT '', file_data BLOB,
 created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
 )`)
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = addGSTColumn(tx, "purchase_invoices", "external_url", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return tx.Commit()
}

func PurchaseEligible(withdrawal float64, head, subhead, number string) bool {
	classification := strings.ToLower(head + " " + subhead)
	return withdrawal > 0 && strings.TrimSpace(number) == "" &&
		!strings.Contains(classification, "salary") && !strings.Contains(classification, "personal") &&
		!strings.EqualFold(strings.TrimSpace(head), "Sales Invoice")
}

func GetPurchaseInvoices(db *sql.DB) ([]PurchaseInvoice, error) {
	rows, err := db.Query(`SELECT id,bank_transaction_id,invoice_number,invoice_date,party_name,party_address,party_gstin,total_amount,file_name,external_url FROM purchase_invoices ORDER BY invoice_date DESC,id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []PurchaseInvoice{}
	for rows.Next() {
		var p PurchaseInvoice
		if err = rows.Scan(&p.ID, &p.BankTransactionID, &p.InvoiceNumber, &p.InvoiceDate, &p.PartyName, &p.PartyAddress, &p.PartyGSTIN, &p.TotalAmount, &p.FileName, &p.ExternalURL); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func SavePurchaseInvoice(db *sql.DB, p *PurchaseInvoice) error {
	if _, err := time.Parse("2006-01-02", p.InvoiceDate); err != nil {
		return fmt.Errorf("Invoice date must be YYYY-MM-DD")
	}
	p.ExternalURL = strings.TrimSpace(p.ExternalURL)
	if p.ExternalURL != "" {
		u, err := url.Parse(p.ExternalURL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil {
			return fmt.Errorf("External invoice URL must be a valid http:// or https:// link without credentials")
		}
	}
	p.PartyName = strings.TrimSpace(p.PartyName)
	p.PartyAddress = strings.TrimSpace(p.PartyAddress)
	p.InvoiceNumber = strings.TrimSpace(p.InvoiceNumber)
	if p.PartyName == "" || p.PartyAddress == "" {
		return fmt.Errorf("Party name and address are required")
	}
	var err error
	p.PartyGSTIN, err = normalizeGSTIN(p.PartyGSTIN)
	if err != nil {
		return err
	}
	if math.IsNaN(p.TotalAmount) || math.IsInf(p.TotalAmount, 0) || p.TotalAmount <= 0 {
		return fmt.Errorf("Total invoice amount must be positive")
	}
	p.TotalAmount = money(p.TotalAmount)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if p.ID > 0 {
		var linked *int
		if err = tx.QueryRow(`SELECT bank_transaction_id FROM purchase_invoices WHERE id=?`, p.ID).Scan(&linked); err != nil {
			return err
		}
		if (linked == nil) != (p.BankTransactionID == nil) || (linked != nil && *linked != *p.BankTransactionID) {
			return fmt.Errorf("The linked withdrawal cannot be changed")
		}
	}
	if p.BankTransactionID != nil {
		var amount float64
		var head, subhead, number string
		if err = tx.QueryRow(`SELECT withdrawal_amt,COALESCE(account_head,''),COALESCE(sub_account_head,''),COALESCE(invoice_number,'') FROM bank_transactions WHERE id=?`, *p.BankTransactionID).Scan(&amount, &head, &subhead, &number); err != nil {
			return err
		}
		if !PurchaseEligible(amount, head, subhead, number) {
			return fmt.Errorf("Only withdrawals without an invoice, excluding Sales Invoice, Salary and Personal entries, can be linked")
		}
		var count int
		if err = tx.QueryRow(`SELECT count(*) FROM purchase_invoices WHERE bank_transaction_id=? AND id<>?`, *p.BankTransactionID, p.ID).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("This withdrawal already has a purchase invoice")
		}
		if err = tx.QueryRow(`SELECT count(*) FROM invoice_allocations WHERE bank_transaction_id=?`, *p.BankTransactionID).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("This withdrawal already has sales allocations")
		}
	}
	if p.ID == 0 {
		result, err := tx.Exec(`INSERT INTO purchase_invoices(bank_transaction_id,invoice_number,invoice_date,party_name,party_address,party_gstin,total_amount,file_name,file_type,file_data,external_url) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, p.BankTransactionID, p.InvoiceNumber, p.InvoiceDate, p.PartyName, p.PartyAddress, p.PartyGSTIN, p.TotalAmount, p.FileName, p.FileType, p.FileData, p.ExternalURL)
		if err != nil {
			return err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		p.ID = int(id)
	} else {
		_, err = tx.Exec(`UPDATE purchase_invoices SET invoice_number=?,invoice_date=?,party_name=?,party_address=?,party_gstin=?,total_amount=?,external_url=? WHERE id=?`, p.InvoiceNumber, p.InvoiceDate, p.PartyName, p.PartyAddress, p.PartyGSTIN, p.TotalAmount, p.ExternalURL, p.ID)
		if err != nil {
			return err
		}
		if p.FileData != nil {
			if _, err = tx.Exec(`UPDATE purchase_invoices SET file_name=?,file_type=?,file_data=? WHERE id=?`, p.FileName, p.FileType, p.FileData, p.ID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
