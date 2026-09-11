package core

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"modernc.org/sqlite"
	"regexp"
	"strings"
	"time"
)

type ReferenceRate struct {
	RateDate    string  `json:"rate_date"`
	Currency    string  `json:"currency"`
	Units       int64   `json:"units"`
	RateINR     float64 `json:"rate_inr"`
	SourceFile  string  `json:"source_file"`
	SourceSheet string  `json:"source_sheet"`
	ImportedAt  string  `json:"imported_at"`
}

var ErrReferenceRateExists = errors.New("a reference rate already exists for that date and currency")
var ErrInvalidReferenceRate = errors.New("invalid reference rate")
var referenceCurrency = regexp.MustCompile(`^[A-Z]{3}$`)

func GetReferenceRates(db *sql.DB) ([]ReferenceRate, error) {
	rows, err := db.Query(`SELECT rate_date,currency,units,rate_inr,source_file,source_sheet,imported_at FROM reference_rates ORDER BY rate_date DESC,currency`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rates := []ReferenceRate{}
	for rows.Next() {
		var rate ReferenceRate
		if err := rows.Scan(&rate.RateDate, &rate.Currency, &rate.Units, &rate.RateINR, &rate.SourceFile, &rate.SourceSheet, &rate.ImportedAt); err != nil {
			return nil, err
		}
		rates = append(rates, rate)
	}
	return rates, rows.Err()
}

// SaveReferenceRate creates a manual rate when original is nil; otherwise updates
// that original composite key, even if the user corrects its date or currency.
func SaveReferenceRate(db *sql.DB, rate ReferenceRate, original *ReferenceRate) (ReferenceRate, error) {
	var empty ReferenceRate
	rate.Currency = strings.ToUpper(strings.TrimSpace(rate.Currency))
	date, err := time.Parse("2006-01-02", rate.RateDate)
	if err != nil || date.Year() < 1 || date.Format("2006-01-02") != rate.RateDate {
		return empty, fmt.Errorf("%w: date must be YYYY-MM-DD", ErrInvalidReferenceRate)
	}
	if !referenceCurrency.MatchString(rate.Currency) || rate.Currency == "INR" {
		return empty, fmt.Errorf("%w: enter a three-letter foreign currency code", ErrInvalidReferenceRate)
	}
	if rate.Units < 1 || rate.Units > 9007199254740991 || rate.RateINR <= 0 || math.IsNaN(rate.RateINR) || math.IsInf(rate.RateINR, 0) {
		return empty, fmt.Errorf("%w: quote units must be a positive safe integer and INR rate a positive finite number", ErrInvalidReferenceRate)
	}
	query := `INSERT INTO reference_rates(rate_date,currency,units,rate_inr,source_file,source_sheet)
      VALUES(?,?,?,?,'Manual entry','')`
	args := []any{rate.RateDate, rate.Currency, rate.Units, rate.RateINR}
	if original != nil {
		query = `UPDATE reference_rates SET rate_date=?,currency=?,units=?,rate_inr=?,source_file='Manual entry',source_sheet='',imported_at=CURRENT_TIMESTAMP WHERE rate_date=? AND currency=?`
		args = append(args, original.RateDate, original.Currency)
	}
	query += ` RETURNING rate_date,currency,units,rate_inr,source_file,source_sheet,imported_at`
	err = db.QueryRow(query, args...).Scan(&rate.RateDate, &rate.Currency, &rate.Units, &rate.RateINR, &rate.SourceFile, &rate.SourceSheet, &rate.ImportedAt)
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) && (sqliteErr.Code() == 1555 || sqliteErr.Code() == 2067) {
		return empty, ErrReferenceRateExists
	}
	return rate, err
}
