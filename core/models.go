package core

// BankTransaction represents a single row from the HDFC bank statement.
type BankTransaction struct {
	ID             int     `json:"id"`
	Date           string  `json:"date"`
	Narration      string  `json:"narration"`
	ChqRefNo       string  `json:"chq_ref_no"`
	ValueDate      string  `json:"value_date"`
	WithdrawalAmt  float64 `json:"withdrawal_amt"`
	DepositAmt     float64 `json:"deposit_amt"`
	ClosingBalance float64 `json:"closing_balance"`

	// Classification fields
	AccountHead         string  `json:"account_head"`
	SubAccountHead      string  `json:"sub_account_head"`
	InvoiceNumber       string  `json:"invoice_number"`
	BusinessPartnerID   *int    `json:"business_partner_id"`
	BusinessPartnerName string  `json:"business_partner_name"`

	// Forex fields
	Currency       string  `json:"currency"`
	ExchangeRate   float64 `json:"exchange_rate"`
	ForexAmount    float64 `json:"forex_amount"`
}

// Classification holds the result of a narration classification rule.
type Classification struct {
	AccountHead    string
	SubAccountHead string
	InvoiceNumber  string
	Currency       string
	ExchangeRate   float64
	ForexAmount    float64
}

// ClassificationRule defines a single narration-matching rule.
// Match is called with the full transaction; if it matches,
// it returns a Classification and true. Rules are evaluated in order;
// first match wins.
type ClassificationRule struct {
	Name  string // Human-readable rule name for logging
	Match func(txn *BankTransaction) (Classification, bool)
}

// AccountMeta holds metadata parsed from the statement header.
type AccountMeta struct {
	AccountNo     string
	CustomerID    string
	Branch        string
	IFSC          string
	MICR          string
	StatementFrom string
	StatementTo   string
}

// BusinessPartner represents a client or vendor configured in the system.
type BusinessPartner struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	BillingAddress  string `json:"billing_address"`
	InvoiceCurrency string `json:"invoice_currency"`
	TaxInformation  string `json:"tax_information"`
}
