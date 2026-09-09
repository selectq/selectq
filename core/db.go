package core

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// ImportRecord represents a record from the imports table.
type ImportRecord struct {
	ID            int    `json:"id"`
	AccountNo     string `json:"account_no"`
	SourceFile    string `json:"source_file"`
	StatementFrom string `json:"statement_from"`
	StatementTo   string `json:"statement_to"`
	ImportedAt    string `json:"imported_at"`
}

// InitDB initializes the database and returns the connection.
func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("create tables: %w", err)
	}
	return db, nil
}

// StoreInDB saves parsed metadata and transactions to the database.
// It returns the number of transactions successfully inserted.
func StoreInDB(db *sql.DB, meta AccountMeta, transactions []BankTransaction, sourceFile string) (int, error) {


	// Insert into a transaction for atomicity
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint: will be committed on success

	// Insert account metadata (upsert on account_no)
	_, err = tx.Exec(`
		INSERT INTO accounts (account_no, customer_id, branch, ifsc, micr)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(account_no) DO UPDATE SET
			customer_id = excluded.customer_id,
			branch = excluded.branch,
			ifsc = excluded.ifsc,
			micr = excluded.micr
	`, meta.AccountNo, meta.CustomerID, meta.Branch, meta.IFSC, meta.MICR)
	if err != nil {
		return 0, fmt.Errorf("insert account: %w", err)
	}

	// Insert import record
	result, err := tx.Exec(`
		INSERT INTO imports (account_no, source_file, statement_from, statement_to)
		VALUES (?, ?, ?, ?)
	`, meta.AccountNo, filepath.Base(sourceFile), meta.StatementFrom, meta.StatementTo)
	if err != nil {
		return 0, fmt.Errorf("insert import: %w", err)
	}
	importID, _ := result.LastInsertId()

	// Prepare transaction insert statement
	stmt, err := tx.Prepare(`
		INSERT INTO bank_transactions (
			import_id, account_no, txn_date, narration, chq_ref_no,
			value_date, withdrawal_amt, deposit_amt, closing_balance,
			account_head, sub_account_head, invoice_number,
			currency, exchange_rate, forex_amount
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare stmt: %w", err)
	}
	defer stmt.Close()

	count := 0
	for _, txn := range transactions {
		_, err := stmt.Exec(
			importID,
			meta.AccountNo,
			txn.Date,
			txn.Narration,
			txn.ChqRefNo,
			txn.ValueDate,
			txn.WithdrawalAmt,
			txn.DepositAmt,
			txn.ClosingBalance,
			txn.AccountHead,
			txn.SubAccountHead,
			txn.InvoiceNumber,
			txn.Currency,
			txn.ExchangeRate,
			txn.ForexAmount,
		)
		if err != nil {
			return 0, fmt.Errorf("insert txn row: %w", err)
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}

	return count, nil
}

// createTables sets up the SQLite schema.
func createTables(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS accounts (
		account_no   TEXT PRIMARY KEY,
		customer_id  TEXT,
		branch       TEXT,
		ifsc         TEXT,
		micr         TEXT,
		created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS imports (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		account_no     TEXT NOT NULL,
		source_file    TEXT NOT NULL,
		statement_from TEXT,
		statement_to   TEXT,
		imported_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (account_no) REFERENCES accounts(account_no)
	);

	CREATE TABLE IF NOT EXISTS bank_transactions (
		id               INTEGER PRIMARY KEY AUTOINCREMENT,
		import_id        INTEGER NOT NULL,
		account_no       TEXT NOT NULL,
		txn_date         TEXT NOT NULL,
		narration        TEXT NOT NULL,
		chq_ref_no       TEXT,
		value_date       TEXT,
		withdrawal_amt   REAL DEFAULT 0,
		deposit_amt      REAL DEFAULT 0,
		closing_balance  REAL DEFAULT 0,
		account_head     TEXT DEFAULT '',
		sub_account_head TEXT DEFAULT '',
		invoice_number   TEXT DEFAULT '',
		currency         TEXT DEFAULT '',
		exchange_rate    REAL DEFAULT 0,
		forex_amount     REAL DEFAULT 0,
		created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (import_id) REFERENCES imports(id),
		FOREIGN KEY (account_no) REFERENCES accounts(account_no)
	);

	CREATE INDEX IF NOT EXISTS idx_txn_account ON bank_transactions(account_no);
	CREATE INDEX IF NOT EXISTS idx_txn_date    ON bank_transactions(txn_date);
	CREATE INDEX IF NOT EXISTS idx_txn_import  ON bank_transactions(import_id);

	CREATE TABLE IF NOT EXISTS business_partners (
		id               INTEGER PRIMARY KEY AUTOINCREMENT,
		name             TEXT UNIQUE NOT NULL,
		billing_address  TEXT,
		invoice_currency TEXT,
		tax_information  TEXT,
		created_at       DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := db.Exec(schema)
	return err
}

// GetImports fetches all import records from the database.
func GetImports(db *sql.DB) ([]ImportRecord, error) {
	rows, err := db.Query(`
		SELECT id, account_no, source_file, statement_from, statement_to, imported_at
		FROM imports ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var imports []ImportRecord
	for rows.Next() {
		var r ImportRecord
		if err := rows.Scan(&r.ID, &r.AccountNo, &r.SourceFile, &r.StatementFrom, &r.StatementTo, &r.ImportedAt); err != nil {
			return nil, err
		}
		imports = append(imports, r)
	}
	return imports, nil
}

// GetTransactions fetches all transactions for a specific import ID.
func GetTransactions(db *sql.DB, importID int) ([]BankTransaction, error) {
	rows, err := db.Query(`
		SELECT txn_date, narration, chq_ref_no, value_date, withdrawal_amt, deposit_amt, closing_balance,
		       account_head, sub_account_head, invoice_number, currency, exchange_rate, forex_amount
		FROM bank_transactions
		WHERE import_id = ?
		ORDER BY id ASC
	`, importID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txns []BankTransaction
	for rows.Next() {
		var t BankTransaction
		if err := rows.Scan(
			&t.Date, &t.Narration, &t.ChqRefNo, &t.ValueDate, &t.WithdrawalAmt, &t.DepositAmt, &t.ClosingBalance,
			&t.AccountHead, &t.SubAccountHead, &t.InvoiceNumber, &t.Currency, &t.ExchangeRate, &t.ForexAmount,
		); err != nil {
			return nil, err
		}
		txns = append(txns, t)
	}
	return txns, nil
}

// CreateBusinessPartner inserts a new business partner.
func CreateBusinessPartner(db *sql.DB, p BusinessPartner) (int64, error) {
	result, err := db.Exec(`
		INSERT INTO business_partners (name, billing_address, invoice_currency, tax_information)
		VALUES (?, ?, ?, ?)
	`, p.Name, p.BillingAddress, p.InvoiceCurrency, p.TaxInformation)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// UpdateBusinessPartner updates an existing business partner.
func UpdateBusinessPartner(db *sql.DB, p BusinessPartner) error {
	_, err := db.Exec(`
		UPDATE business_partners 
		SET name = ?, billing_address = ?, invoice_currency = ?, tax_information = ?
		WHERE id = ?
	`, p.Name, p.BillingAddress, p.InvoiceCurrency, p.TaxInformation, p.ID)
	return err
}

// GetBusinessPartnerByID fetches a single business partner by ID.
func GetBusinessPartnerByID(db *sql.DB, id int) (BusinessPartner, error) {
	var p BusinessPartner
	err := db.QueryRow(`
		SELECT id, name, billing_address, invoice_currency, tax_information
		FROM business_partners WHERE id = ?
	`, id).Scan(&p.ID, &p.Name, &p.BillingAddress, &p.InvoiceCurrency, &p.TaxInformation)
	return p, err
}

// GetBusinessPartners fetches business partners, optionally filtering by name.
func GetBusinessPartners(db *sql.DB, query string) ([]BusinessPartner, error) {
	sqlQuery := `
		SELECT id, name, billing_address, invoice_currency, tax_information
		FROM business_partners
	`
	var rows *sql.Rows
	var err error

	if query != "" {
		sqlQuery += " WHERE name LIKE ? ORDER BY name ASC"
		rows, err = db.Query(sqlQuery, "%"+query+"%")
	} else {
		sqlQuery += " ORDER BY name ASC"
		rows, err = db.Query(sqlQuery)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var partners []BusinessPartner
	for rows.Next() {
		var p BusinessPartner
		if err := rows.Scan(&p.ID, &p.Name, &p.BillingAddress, &p.InvoiceCurrency, &p.TaxInformation); err != nil {
			return nil, err
		}
		partners = append(partners, p)
	}
	return partners, nil
}
