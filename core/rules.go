package core

import (
	"regexp"
	"strconv"
	"strings"
)

// ------------------------------------------------------------------
// Classification Rules Registry
// ------------------------------------------------------------------
// Add new rules here. They are evaluated top-to-bottom; first match wins.
// Each rule is a function: narration string -> (Classification, matched bool)

var classificationRules = []ClassificationRule{
	{
		Name: "CGST/SGST Bank Charges",
		Match: func(txn *BankTransaction) (Classification, bool) {
			// Pattern: "<ref> DPO<invoice> CGST" or "<ref> DPO<invoice> SGST"
			// Example: "100726I049901572 DPO2719109788310 CGST"
			re := regexp.MustCompile(`(DPO\w+)\s+(CGST|SGST)\s*$`)
			matches := re.FindStringSubmatch(txn.Narration)
			if matches == nil {
				return Classification{}, false
			}
			return Classification{
				AccountHead:    "Bank Charges",
				SubAccountHead: matches[2], // "CGST" or "SGST"
				InvoiceNumber:  matches[1], // "DPO2719109788310"
			}, true
		},
	},

	{
		Name: "Inward Remittance (Sales Invoice)",
		Match: func(txn *BankTransaction) (Classification, bool) {
			// Pattern: "INW <ref> <CCY><amount>@<exchange_rate>"
			// Examples:
			//   "INW 100726I049901572 USD5056.0@93.59"
			//   "INW 310726I049905243 GBP1450.0@124.74"
			re := regexp.MustCompile(
				`^INW\s+(\S+)\s+([A-Z]{3})([\d.]+)@([\d.]+)\s*$`,
			)
			matches := re.FindStringSubmatch(txn.Narration)
			if matches == nil {
				return Classification{}, false
			}
			// matches[1] = ref (e.g., "100726I049901572")
			// matches[2] = currency code (e.g., "USD")
			// matches[3] = forex amount (e.g., "5056.0")
			// matches[4] = exchange rate (e.g., "93.59")
			forexAmt, _ := strconv.ParseFloat(matches[3], 64)
			exchRate, _ := strconv.ParseFloat(matches[4], 64)

			return Classification{
				AccountHead:    "Sales Invoice",
				SubAccountHead: "",
				InvoiceNumber:  matches[1],
				Currency:       matches[2],
				ExchangeRate:   exchRate,
				ForexAmount:    forexAmt,
			}, true
		},
	},

	{
		Name: "Personal Transfer (Ritu Gandhi)",
		Match: func(txn *BankTransaction) (Classification, bool) {
			// Pattern: "00891050087191-TPT-TO PERSONAL-RITU GANDHI"
			if !strings.Contains(txn.Narration, "00891050087191") {
				return Classification{}, false
			}
			return Classification{
				AccountHead:    "To Personal",
				SubAccountHead: "Ritu Gandhi",
			}, true
		},
	},

	{
		Name: "Salary Payments",
		Match: func(txn *BankTransaction) (Classification, bool) {
			if txn.WithdrawalAmt <= 0 {
				return Classification{}, false
			}
			// Pattern: "NEFT DR-<IFSC>-<EMPLOYEE_NAME>-NETBANK, MUM-<REF>-SALARY <MONTH> <YEAR>"
			// Example: "NEFT DR-CBIN0283643-SNEHA-NETBANK, MUM-HDFCH01122159998-SALARY JUNE 26"
			re := regexp.MustCompile(`^NEFT DR-\S+-(.+?)-NETBANK.*-SALARY`)
			matches := re.FindStringSubmatch(txn.Narration)
			if matches == nil {
				return Classification{}, false
			}
			// Title-case the employee name (e.g., "NITIN RAWAT" -> "Nitin Rawat")
			name := strings.Title(strings.ToLower(strings.TrimSpace(matches[1])))
			return Classification{
				AccountHead:    "Salary",
				SubAccountHead: name,
			}, true
		},
	},

	{
		Name: "Tax Payments",
		Match: func(txn *BankTransaction) (Classification, bool) {
			if strings.Contains(txn.Narration, "CBDT TIN") {
				return Classification{AccountHead: "Tax", SubAccountHead: "Income Tax"}, true
			}
			if strings.Contains(txn.Narration, "GOODS AND SERVIE TAX") {
				return Classification{AccountHead: "Tax", SubAccountHead: "GST"}, true
			}
			return Classification{}, false
		},
	},

	{
		Name: "Software and Services",
		Match: func(txn *BankTransaction) (Classification, bool) {
			if strings.Contains(txn.Narration, "PLAYSTOREGOOGL") {
				return Classification{AccountHead: "Software/Services", SubAccountHead: "Google Play"}, true
			}
			if strings.Contains(txn.Narration, "GODADDY") {
				return Classification{AccountHead: "Software/Services", SubAccountHead: "GoDaddy"}, true
			}
			return Classification{}, false
		},
	},

	{
		Name: "Travel and Hotels",
		Match: func(txn *BankTransaction) (Classification, bool) {
			if strings.Contains(txn.Narration, "AIRINDIAEXPRESS") || strings.Contains(txn.Narration, "INDIGO AIRLINE") {
				return Classification{AccountHead: "Travel", SubAccountHead: "Flights"}, true
			}
			if strings.Contains(txn.Narration, "DOUBLE TREE BY HILTON") {
				return Classification{AccountHead: "Travel", SubAccountHead: "Hotel"}, true
			}
			return Classification{}, false
		},
	},

	{
		Name: "Insurance",
		Match: func(txn *BankTransaction) (Classification, bool) {
			if strings.Contains(txn.Narration, "TATAAIGGENERALIN") {
				return Classification{AccountHead: "Insurance", SubAccountHead: "Tata AIG"}, true
			}
			return Classification{}, false
		},
	},

	{
		Name: "Fuel",
		Match: func(txn *BankTransaction) (Classification, bool) {
			if strings.Contains(txn.Narration, "FILLING STATION") {
				return Classification{AccountHead: "Fuel", SubAccountHead: ""}, true
			}
			return Classification{}, false
		},
	},

	{
		Name: "Inward Domestic Transfers",
		Match: func(txn *BankTransaction) (Classification, bool) {
			if strings.Contains(txn.Narration, "HBA STUDIO") {
				return Classification{AccountHead: "Domestic Sales", SubAccountHead: "HBA Studio"}, true
			}
			return Classification{}, false
		},
	},

	{
		Name: "Misc Expenses",
		Match: func(txn *BankTransaction) (Classification, bool) {
			if strings.Contains(txn.Narration, "VISHU") {
				return Classification{AccountHead: "Misc Expenses", SubAccountHead: "Vishu"}, true
			}
			return Classification{}, false
		},
	},

	// ---------------------------------------------------------------
	// ADD NEW RULES HERE
	// ---------------------------------------------------------------
}

// ClassifyTransaction attempts to apply all registered rules to the given transaction.
func ClassifyTransaction(txn *BankTransaction) {
	for _, rule := range classificationRules {
		if cls, ok := rule.Match(txn); ok {
			txn.AccountHead = cls.AccountHead
			txn.SubAccountHead = cls.SubAccountHead
			txn.InvoiceNumber = cls.InvoiceNumber
			txn.Currency = cls.Currency
			txn.ExchangeRate = cls.ExchangeRate
			txn.ForexAmount = cls.ForexAmount
			return
		}
	}
	// No rule matched — fields remain empty
}
