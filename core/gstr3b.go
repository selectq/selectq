package core

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"time"
)

type GSTR3BRow struct {
	InvoiceNumber string   `json:"invoice_number"`
	Currency      string   `json:"currency"`
	Amount        float64  `json:"amount"`
	InvoicedTo    string   `json:"invoiced_to"`
	Address       string   `json:"address"`
	InvoiceDate   string   `json:"invoice_date"`
	ExchangeRate  *float64 `json:"exchange_rate"`
	ValueINR      *float64 `json:"value_inr"`
	ReferenceDate *string  `json:"reference_date"`
}

type GSTR3BSummary struct {
	FinancialYear  string      `json:"financial_year"`
	FinancialYears []string    `json:"financial_years"`
	Rows           []GSTR3BRow `json:"rows"`
}

func ValidFinancialYear(fy string) bool {
	if len(fy) != 9 {
		return false
	}
	date, err := time.Parse("2006-01-02", fy[:4]+"-04-01")
	return err == nil && date.Year() > 0 && fy == fmt.Sprintf("%04d-%04d", date.Year(), date.Year()+1)
}

// Rates are matched per currency on or before the invoice date, across FY boundaries.
func GetGSTR3BSummary(db *sql.DB, fy string) (GSTR3BSummary, error) {
	result := GSTR3BSummary{FinancialYear: fy, FinancialYears: []string{}, Rows: []GSTR3BRow{}}
	rows, err := db.Query(`SELECT s.invoice_number, upper(trim(s.currency)), s.amount,
 COALESCE(bp.name,''), COALESCE(a.address,''), s.invoice_date,
 r.rate_inr, r.units, r.rate_date,
 COALESCE((SELECT SUM(li.amount * li.gst_percent / 100)
 FROM sales_invoice_line_items li WHERE li.sales_invoice_id=s.id),0)
 FROM sales_invoices s
 LEFT JOIN business_partners bp ON bp.id=s.business_partner_id
 LEFT JOIN bp_addresses a ON a.id=s.address_id
 LEFT JOIN reference_rates r ON r.currency=upper(trim(s.currency)) AND r.rate_date=(
 SELECT max(rr.rate_date) FROM reference_rates rr WHERE rr.currency=upper(trim(s.currency)) AND rr.rate_date<=s.invoice_date)
 ORDER BY s.invoice_number ASC, s.id ASC`)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	years := map[string]bool{}
	for rows.Next() {
		var row GSTR3BRow
		var rate, units sql.NullFloat64
		var date sql.NullString
		var gstAmount float64
		if err := rows.Scan(&row.InvoiceNumber, &row.Currency, &row.Amount, &row.InvoicedTo, &row.Address, &row.InvoiceDate, &rate, &units, &date, &gstAmount); err != nil {
			return result, err
		}
		year, err := InvoiceFinancialYear(row.InvoiceDate)
		if err != nil {
			return result, err
		}
		years[year] = true
		if fy != "" && year != fy {
			continue
		}
		if row.Currency == "INR" {
			// Include the invoice GST total, rounded as in populateInvoiceGST.
			row.Amount = money(row.Amount + money(gstAmount))
			one := 1.0
			row.ExchangeRate = &one
		} else if rate.Valid && units.Valid && units.Float64 > 0 {
			value := rate.Float64 / units.Float64
			row.ExchangeRate = &value
			row.ReferenceDate = &date.String
		}
		if row.ExchangeRate != nil {
			value := math.Round(row.Amount**row.ExchangeRate*100) / 100
			row.ValueINR = &value
		}
		result.Rows = append(result.Rows, row)
	}
	for year := range years {
		result.FinancialYears = append(result.FinancialYears, year)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(result.FinancialYears)))
	return result, rows.Err()
}
