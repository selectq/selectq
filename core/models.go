package core

// BankTransaction represents a single row from the HDFC bank statement.
type BankTransaction struct {
	PurchaseInvoiceID *int    `json:"purchase_invoice_id"`
	ID                int     `json:"id"`
	Date              string  `json:"date"`
	Narration         string  `json:"narration"`
	ChqRefNo          string  `json:"chq_ref_no"`
	ValueDate         string  `json:"value_date"`
	WithdrawalAmt     float64 `json:"withdrawal_amt"`
	DepositAmt        float64 `json:"deposit_amt"`
	ClosingBalance    float64 `json:"closing_balance"`

	Allocations []InvoiceAllocation `json:"allocations"`
	// Classification fields
	AccountHead         string `json:"account_head"`
	SubAccountHead      string `json:"sub_account_head"`
	InvoiceNumber       string `json:"invoice_number"`
	BusinessPartnerID   *int   `json:"business_partner_id"`
	BusinessPartnerName string `json:"business_partner_name"`

	// Forex fields
	Currency     string  `json:"currency"`
	ExchangeRate float64 `json:"exchange_rate"`
	ForexAmount  float64 `json:"forex_amount"`
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

// BusinessPartnerContact represents a single contact for a BusinessPartner.
type BusinessPartnerContact struct {
	ID                int    `json:"id"`
	BusinessPartnerID int    `json:"business_partner_id"`
	Name              string `json:"name"`
	Email             string `json:"email"`
	Phone             string `json:"phone"`
	IsPrimary         bool   `json:"is_primary"`
}

// BusinessPartner represents a client or vendor configured in the system.
type BusinessPartner struct {
	Addresses       []BPAddress              `json:"addresses"`
	ID              int                      `json:"id"`
	Name            string                   `json:"name"`
	BillingAddress  string                   `json:"billing_address"`
	InvoiceCurrency string                   `json:"invoice_currency"`
	TaxInformation  string                   `json:"tax_information"`
	Contacts        []BusinessPartnerContact `json:"contacts"`
}

// SalesInvoice represents a sales invoice stored in the system.
type SalesInvoice struct {
	SellerGSTIN         string            `json:"seller_gstin"`
	BuyerGSTIN          string            `json:"buyer_gstin"`
	GSTTreatment        string            `json:"gst_treatment"`
	GSTAmount           float64           `json:"gst_amount"`
	CGSTAmount          float64           `json:"cgst_amount"`
	SGSTAmount          float64           `json:"sgst_amount"`
	IGSTAmount          float64           `json:"igst_amount"`
	AddressID           *int              `json:"address_id"`
	BillingAddress      string            `json:"billing_address"`
	IsClosed            bool              `json:"is_closed"`
	ExpectedReceipt     float64           `json:"expected_receipt"`
	ReceivedAmount      float64           `json:"received_amount"`
	OutstandingAmount   float64           `json:"outstanding_amount"`
	IsSettled           bool              `json:"is_settled"`
	ID                  int               `json:"id"`
	InvoiceNumber       string            `json:"invoice_number"`
	FinancialYear       string            `json:"financial_year"`
	BusinessPartnerID   int               `json:"business_partner_id"`
	BusinessPartnerName string            `json:"business_partner_name,omitempty"`
	ContactID           *int              `json:"contact_id,omitempty"`
	InvoiceDate         string            `json:"invoice_date"`
	DueInDays           int               `json:"due_in_days"`
	DueDate             string            `json:"due_date"`
	Currency            string            `json:"currency"`
	Amount              float64           `json:"amount"`
	LineItems           []InvoiceLineItem `json:"line_items,omitempty"`
}

// InvoiceLineItem represents a single line item on a sales invoice.
type InvoiceLineItem struct {
	CGSTPercent    float64 `json:"cgst_percent"`
	SGSTPercent    float64 `json:"sgst_percent"`
	IGSTPercent    float64 `json:"igst_percent"`
	ID             int     `json:"id"`
	SalesInvoiceID int     `json:"sales_invoice_id"`
	Description    string  `json:"description"`
	HsnSacCode     string  `json:"hsn_sac_code"`
	Quantity       float64 `json:"quantity"`
	Rate           float64 `json:"rate"`
	GstPercent     float64 `json:"gst_percent"`
	Amount         float64 `json:"amount"`
}

// CompanyProfile holds the seller/company information for invoices.
type CompanyProfile struct {
	ID          int    `json:"id"`
	CompanyName string `json:"company_name"`
	Address     string `json:"address"`
	GSTIN       string `json:"gstin"`
	PAN         string `json:"pan"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
}
