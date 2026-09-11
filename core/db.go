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

	// Enable foreign key enforcement — SQLite has this OFF by default.
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("create tables: %w", err)
	}
	if err := repairBlankInvoiceNumbers(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("repair invoice numbers: %w", err)
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

	CREATE TABLE IF NOT EXISTS business_partner_contacts (
		id                  INTEGER PRIMARY KEY AUTOINCREMENT,
		business_partner_id INTEGER NOT NULL,
		name                TEXT NOT NULL,
		email               TEXT DEFAULT '',
		phone               TEXT DEFAULT '',
		is_primary          BOOLEAN DEFAULT 0,
		FOREIGN KEY (business_partner_id) REFERENCES business_partners(id) ON DELETE CASCADE
	);

	CREATE UNIQUE INDEX IF NOT EXISTS idx_bpc_email_unique 
	ON business_partner_contacts(email) 
	WHERE email != '';

	CREATE TABLE IF NOT EXISTS invoice_sequences (
		financial_year TEXT PRIMARY KEY,
		last_number INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS sales_invoices (
		id                  INTEGER PRIMARY KEY AUTOINCREMENT,
		invoice_number      TEXT NOT NULL UNIQUE,
		financial_year      TEXT NOT NULL,
		business_partner_id INTEGER NOT NULL,
		contact_id          INTEGER,
		invoice_date        TEXT NOT NULL,
		due_in_days         INTEGER DEFAULT 0,
		due_date            TEXT DEFAULT '',
		currency            TEXT NOT NULL,
		amount              REAL NOT NULL DEFAULT 0,
		created_at          DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (business_partner_id) REFERENCES business_partners(id),
		FOREIGN KEY (contact_id) REFERENCES business_partner_contacts(id)
	);

	CREATE TABLE IF NOT EXISTS sales_invoice_line_items (
		id                 INTEGER PRIMARY KEY AUTOINCREMENT,
		sales_invoice_id   INTEGER NOT NULL,
		description        TEXT NOT NULL,
		hsn_sac_code       TEXT DEFAULT '',
		quantity           REAL DEFAULT 1,
		rate               REAL DEFAULT 0,
		gst_percent        REAL DEFAULT 0,
		amount             REAL NOT NULL DEFAULT 0,
		created_at         DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (sales_invoice_id) REFERENCES sales_invoices(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS company_profile (
		id           INTEGER PRIMARY KEY CHECK (id = 1),
		company_name TEXT DEFAULT '',
		address      TEXT DEFAULT '',
		gstin        TEXT DEFAULT '',
		pan          TEXT DEFAULT '',
		email        TEXT DEFAULT '',
		phone        TEXT DEFAULT ''
	);

	INSERT OR IGNORE INTO company_profile (id) VALUES (1);
	`
	_, err := db.Exec(schema)
	
	// Add columns safely (sqlite ALTER TABLE ADD COLUMN does not support IF NOT EXISTS natively, but errors can be ignored safely if table already has them).
	db.Exec("ALTER TABLE bank_transactions ADD COLUMN business_partner_id INTEGER REFERENCES business_partners(id);")
	db.Exec("ALTER TABLE sales_invoices ADD COLUMN contact_id INTEGER REFERENCES business_partner_contacts(id);")
	db.Exec("ALTER TABLE sales_invoices ADD COLUMN due_in_days INTEGER DEFAULT 0;")
	db.Exec("ALTER TABLE sales_invoices ADD COLUMN due_date TEXT DEFAULT '';")
	
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
		SELECT t.id, t.txn_date, t.narration, t.chq_ref_no, t.value_date, t.withdrawal_amt, t.deposit_amt, t.closing_balance,
		       t.account_head, t.sub_account_head, t.invoice_number, t.business_partner_id, bp.name, t.currency, t.exchange_rate, t.forex_amount
		FROM bank_transactions t
		LEFT JOIN business_partners bp ON t.business_partner_id = bp.id
		WHERE t.import_id = ?
		ORDER BY t.id ASC
	`, importID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txns []BankTransaction
	for rows.Next() {
		var t BankTransaction
		var bpID sql.NullInt64
		var bpName sql.NullString
		
		if err := rows.Scan(
			&t.ID, &t.Date, &t.Narration, &t.ChqRefNo, &t.ValueDate, &t.WithdrawalAmt, &t.DepositAmt, &t.ClosingBalance,
			&t.AccountHead, &t.SubAccountHead, &t.InvoiceNumber, &bpID, &bpName, &t.Currency, &t.ExchangeRate, &t.ForexAmount,
		); err != nil {
			return nil, err
		}
		
		if bpID.Valid {
			id := int(bpID.Int64)
			t.BusinessPartnerID = &id
			t.BusinessPartnerName = bpName.String
		}
		
		txns = append(txns, t)
	}
	return txns, nil
}

// UpdateTransactionClassification saves edits to a transaction's classification.
func UpdateTransactionClassification(db *sql.DB, txnID int, accountHead, subAccountHead, invoiceNumber string, bpID *int) error {
	var err error
	if bpID != nil {
		_, err = db.Exec(`
			UPDATE bank_transactions
			SET account_head = ?, sub_account_head = ?, invoice_number = ?, business_partner_id = ?
			WHERE id = ?
		`, accountHead, subAccountHead, invoiceNumber, *bpID, txnID)
	} else {
		_, err = db.Exec(`
			UPDATE bank_transactions
			SET account_head = ?, sub_account_head = ?, invoice_number = ?, business_partner_id = NULL
			WHERE id = ?
		`, accountHead, subAccountHead, invoiceNumber, txnID)
	}
	return err
}

// CreateBusinessPartner inserts a new business partner.
func CreateBusinessPartner(db *sql.DB, p BusinessPartner) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
		INSERT INTO business_partners (name, billing_address, invoice_currency, tax_information)
		VALUES (?, ?, ?, ?)
	`, p.Name, p.BillingAddress, p.InvoiceCurrency, p.TaxInformation)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	if err := saveBusinessPartnerContacts(tx, int(id), p.Contacts); err != nil {
		return 0, fmt.Errorf("saving contacts: %w", err)
	}

	return id, tx.Commit()
}

// UpdateBusinessPartner updates an existing business partner.
func UpdateBusinessPartner(db *sql.DB, p BusinessPartner) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		UPDATE business_partners 
		SET name = ?, billing_address = ?, invoice_currency = ?, tax_information = ?
		WHERE id = ?
	`, p.Name, p.BillingAddress, p.InvoiceCurrency, p.TaxInformation, p.ID)
	if err != nil {
		return err
	}

	if err := saveBusinessPartnerContacts(tx, p.ID, p.Contacts); err != nil {
		return fmt.Errorf("saving contacts: %w", err)
	}

	return tx.Commit()
}

func saveBusinessPartnerContacts(tx *sql.Tx, bpID int, contacts []BusinessPartnerContact) error {
	// Simple approach: delete existing contacts, re-insert them.
	// (SQLite enforces ON DELETE CASCADE if enabled, but we do it manually to be safe if PRAGMA was skipped).
	_, err := tx.Exec(`DELETE FROM business_partner_contacts WHERE business_partner_id = ?`, bpID)
	if err != nil {
		return err
	}

	for _, c := range contacts {
		_, err := tx.Exec(`
			INSERT INTO business_partner_contacts (business_partner_id, name, email, phone, is_primary)
			VALUES (?, ?, ?, ?, ?)
		`, bpID, c.Name, c.Email, c.Phone, c.IsPrimary)
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteBusinessPartner deletes a business partner by ID.
// It refuses deletion if there are linked transactions or sales invoices.
func DeleteBusinessPartner(db *sql.DB, id int) error {
	// Check for linked bank transactions
	var txnCount int
	err := db.QueryRow(`SELECT COUNT(*) FROM bank_transactions WHERE business_partner_id = ?`, id).Scan(&txnCount)
	if err != nil {
		return fmt.Errorf("checking transactions: %w", err)
	}

	// Check for linked sales invoices
	var invCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM sales_invoices WHERE business_partner_id = ?`, id).Scan(&invCount)
	if err != nil {
		return fmt.Errorf("checking invoices: %w", err)
	}

	if txnCount > 0 || invCount > 0 {
		parts := []string{}
		if txnCount > 0 {
			parts = append(parts, fmt.Sprintf("%d transaction(s)", txnCount))
		}
		if invCount > 0 {
			parts = append(parts, fmt.Sprintf("%d sales invoice(s)", invCount))
		}
		msg := "Cannot delete: partner is linked to "
		for i, p := range parts {
			if i > 0 {
				msg += " and "
			}
			msg += p
		}
		return fmt.Errorf("%s", msg)
	}

	// Note: We don't need to manually delete contacts if PRAGMA foreign_keys = ON and ON DELETE CASCADE is set,
	// but let's delete them explicitly to be safe.
	_, _ = db.Exec(`DELETE FROM business_partner_contacts WHERE business_partner_id = ?`, id)

	_, err = db.Exec(`DELETE FROM business_partners WHERE id = ?`, id)
	return err
}

// GetBusinessPartnerByID fetches a single business partner by ID.
func GetBusinessPartnerByID(db *sql.DB, id int) (BusinessPartner, error) {
	var p BusinessPartner
	err := db.QueryRow(`
		SELECT id, name, billing_address, invoice_currency, tax_information
		FROM business_partners WHERE id = ?
	`, id).Scan(&p.ID, &p.Name, &p.BillingAddress, &p.InvoiceCurrency, &p.TaxInformation)
	
	if err == nil {
		p.Contacts, err = getBusinessPartnerContacts(db, p.ID)
	}
	
	return p, err
}

func getBusinessPartnerContacts(db *sql.DB, bpID int) ([]BusinessPartnerContact, error) {
	rows, err := db.Query(`
		SELECT id, business_partner_id, name, email, phone, is_primary
		FROM business_partner_contacts WHERE business_partner_id = ?
	`, bpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	contacts := make([]BusinessPartnerContact, 0)
	for rows.Next() {
		var c BusinessPartnerContact
		var isPrimary int
		if err := rows.Scan(&c.ID, &c.BusinessPartnerID, &c.Name, &c.Email, &c.Phone, &isPrimary); err == nil {
			c.IsPrimary = (isPrimary != 0)
			contacts = append(contacts, c)
		} else {
			return nil, fmt.Errorf("reading contact for partner %d: %w", bpID, err)
		}
	}
	return contacts, rows.Err()
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range partners {
		partners[i].Contacts, err = getBusinessPartnerContacts(db, partners[i].ID)
		if err != nil {
			return nil, fmt.Errorf("loading contacts for partner %d: %w", partners[i].ID, err)
		}
	}
	return partners, nil
}

// GetSalesInvoices fetches all sales invoices with their line items.
func GetSalesInvoices(db *sql.DB) ([]SalesInvoice, error) {
	rows, err := db.Query(`
		SELECT s.id, s.invoice_number, s.financial_year, s.business_partner_id, bp.name, 
		       s.contact_id, s.invoice_date, s.due_in_days, s.due_date, s.currency, s.amount
		FROM sales_invoices s
		LEFT JOIN business_partners bp ON s.business_partner_id = bp.id
		ORDER BY s.invoice_date DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invoices []SalesInvoice
	for rows.Next() {
		var inv SalesInvoice
		var bpName sql.NullString
		var contactID sql.NullInt64
		if err := rows.Scan(
			&inv.ID, &inv.InvoiceNumber, &inv.FinancialYear, &inv.BusinessPartnerID, &bpName,
			&contactID, &inv.InvoiceDate, &inv.DueInDays, &inv.DueDate, &inv.Currency, &inv.Amount,
		); err != nil {
			return nil, err
		}
		if bpName.Valid {
			inv.BusinessPartnerName = bpName.String
		}
		if contactID.Valid {
			id := int(contactID.Int64)
			inv.ContactID = &id
		}
		// Load line items for this invoice
		items, err := GetLineItems(db, inv.ID)
		if err != nil {
			return nil, fmt.Errorf("loading line items for invoice %d: %w", inv.ID, err)
		}
		inv.LineItems = items
		invoices = append(invoices, inv)
	}
	return invoices, nil
}

// GetSalesInvoiceByID fetches a single sales invoice by ID with its line items.
func GetSalesInvoiceByID(db *sql.DB, id int) (SalesInvoice, error) {
	var inv SalesInvoice
	var bpName sql.NullString
	var contactID sql.NullInt64
	err := db.QueryRow(`
		SELECT s.id, s.invoice_number, s.financial_year, s.business_partner_id, bp.name, 
		       s.contact_id, s.invoice_date, s.due_in_days, s.due_date, s.currency, s.amount
		FROM sales_invoices s
		LEFT JOIN business_partners bp ON s.business_partner_id = bp.id
		WHERE s.id = ?
	`, id).Scan(
		&inv.ID, &inv.InvoiceNumber, &inv.FinancialYear, &inv.BusinessPartnerID, &bpName,
		&contactID, &inv.InvoiceDate, &inv.DueInDays, &inv.DueDate, &inv.Currency, &inv.Amount,
	)
	if err != nil {
		return inv, err
	}
	if bpName.Valid {
		inv.BusinessPartnerName = bpName.String
	}
	if contactID.Valid {
		idVal := int(contactID.Int64)
		inv.ContactID = &idVal
	}
	items, err := GetLineItems(db, inv.ID)
	if err != nil {
		return inv, err
	}
	inv.LineItems = items
	return inv, nil
}

// GetLineItems fetches all line items for a given sales invoice.
func GetLineItems(db *sql.DB, invoiceID int) ([]InvoiceLineItem, error) {
	rows, err := db.Query(`
		SELECT id, sales_invoice_id, description, hsn_sac_code, quantity, rate, gst_percent, amount
		FROM sales_invoice_line_items
		WHERE sales_invoice_id = ?
		ORDER BY id
	`, invoiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []InvoiceLineItem
	for rows.Next() {
		var li InvoiceLineItem
		if err := rows.Scan(&li.ID, &li.SalesInvoiceID, &li.Description, &li.HsnSacCode, &li.Quantity, &li.Rate, &li.GstPercent, &li.Amount); err != nil {
			return nil, err
		}
		items = append(items, li)
	}
	return items, nil
}

// SaveLineItems replaces all line items for an invoice and updates the invoice total.
func SaveLineItems(db *sql.DB, invoiceID int, items []InvoiceLineItem) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete existing line items
	if _, err := tx.Exec(`DELETE FROM sales_invoice_line_items WHERE sales_invoice_id = ?`, invoiceID); err != nil {
		return err
	}

	// Insert new line items and compute total
	var total float64
	for _, item := range items {
		_, err := tx.Exec(`
			INSERT INTO sales_invoice_line_items (sales_invoice_id, description, hsn_sac_code, quantity, rate, gst_percent, amount)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, invoiceID, item.Description, item.HsnSacCode, item.Quantity, item.Rate, item.GstPercent, item.Amount)
		if err != nil {
			return err
		}
		total += item.Amount
	}

	// Update invoice total
	if _, err := tx.Exec(`UPDATE sales_invoices SET amount = ? WHERE id = ?`, total, invoiceID); err != nil {
		return err
	}

	return tx.Commit()
}

// CreateSalesInvoice creates a new sales invoice with its line items.
func CreateSalesInvoice(db *sql.DB, inv SalesInvoice) (int, error) {
	fy, err := InvoiceFinancialYear(inv.InvoiceDate)
	if err != nil {
		return 0, err
	}
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	inv.InvoiceNumber, err = nextInvoiceNumber(tx, fy)
	if err != nil {
		return 0, err
	}
	inv.FinancialYear = fy
	// Compute total from line items
	var total float64
	for _, item := range inv.LineItems {
		total += item.Amount
	}

	result, err := tx.Exec(`
		INSERT INTO sales_invoices (invoice_number, financial_year, business_partner_id, contact_id, invoice_date, due_in_days, due_date, currency, amount)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, inv.InvoiceNumber, inv.FinancialYear, inv.BusinessPartnerID, inv.ContactID, inv.InvoiceDate, inv.DueInDays, inv.DueDate, inv.Currency, total)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	for _, item := range inv.LineItems {
		if _, err := tx.Exec(`INSERT INTO sales_invoice_line_items
			(sales_invoice_id, description, hsn_sac_code, quantity, rate, gst_percent, amount)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, id, item.Description, item.HsnSacCode, item.Quantity, item.Rate, item.GstPercent, item.Amount); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return int(id), nil
}

// UpdateSalesInvoice updates an existing sales invoice and its line items.
func UpdateSalesInvoice(db *sql.DB, inv SalesInvoice) error {
	fy, err := InvoiceFinancialYear(inv.InvoiceDate)
	if err != nil {
		return err
	}
	var originalDate string
	if err := db.QueryRow(`SELECT invoice_date FROM sales_invoices WHERE id = ?`, inv.ID).Scan(&originalDate); err != nil {
		return err
	}
	originalFY, err := InvoiceFinancialYear(originalDate)
	if err != nil {
		return err
	}
	if fy != originalFY {
		return fmt.Errorf("invoice date must remain within financial year %s; create a new invoice for another financial year", originalFY)
	}
	// Compute total from line items
	var total float64
	for _, item := range inv.LineItems {
		total += item.Amount
	}

	_, err = db.Exec(`
		UPDATE sales_invoices
		SET business_partner_id = ?, contact_id = ?, invoice_date = ?, due_in_days = ?, due_date = ?, currency = ?, amount = ?
		WHERE id = ?
	`, inv.BusinessPartnerID, inv.ContactID, inv.InvoiceDate, inv.DueInDays, inv.DueDate, inv.Currency, total, inv.ID)
	if err != nil {
		return err
	}

	// Replace line items
	if err := SaveLineItems(db, inv.ID, inv.LineItems); err != nil {
		return fmt.Errorf("saving line items: %w", err)
	}

	return nil
}

// GetCompanyProfile fetches the singleton company profile.
func GetCompanyProfile(db *sql.DB) (CompanyProfile, error) {
	var p CompanyProfile
	err := db.QueryRow(`
		SELECT id, company_name, address, gstin, pan, email, phone
		FROM company_profile WHERE id = 1
	`).Scan(&p.ID, &p.CompanyName, &p.Address, &p.GSTIN, &p.PAN, &p.Email, &p.Phone)
	return p, err
}

// SaveCompanyProfile updates the singleton company profile.
func SaveCompanyProfile(db *sql.DB, p CompanyProfile) error {
	_, err := db.Exec(`
		UPDATE company_profile
		SET company_name = ?, address = ?, gstin = ?, pan = ?, email = ?, phone = ?
		WHERE id = 1
	`, p.CompanyName, p.Address, p.GSTIN, p.PAN, p.Email, p.Phone)
	return err
}
