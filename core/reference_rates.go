package core

import (
	"database/sql"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

func initReferenceRates(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS reference_rates (
 rate_date TEXT NOT NULL CHECK(length(rate_date)=10),
 currency TEXT NOT NULL CHECK(currency GLOB '[A-Z][A-Z][A-Z]' AND currency<>'INR'),
 units INTEGER NOT NULL CHECK(units>0),
 rate_inr REAL NOT NULL CHECK(rate_inr>0),
 source_file TEXT NOT NULL,
 source_sheet TEXT NOT NULL,
 imported_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY(rate_date,currency)
);`)
	return err
}

type ReferenceRateImport struct {
	EmptySkipped int    `json:"empty_skipped"`
	Sheet        string `json:"sheet"`
	Inserted     int    `json:"inserted"`
	Skipped      int    `json:"skipped"`
	FromDate     string `json:"from_date"`
	ToDate       string `json:"to_date"`
}

var referenceRateHeader = regexp.MustCompile(`^([A-Z]{3}) \(INR / ([1-9][0-9]*) ([A-Z]{3})\)$`)

// ImportReferenceRates imports quoted INR rates without rounding or converting
// their units. All rows commit together. Identical repeats are skipped, while
// conflicting dates/currencies fail instead of silently replacing reference data.
func ImportReferenceRates(db *sql.DB, filename, sheet string) (ReferenceRateImport, error) {
	var empty ReferenceRateImport
	f, err := excelize.OpenFile(filename)
	if err != nil {
		return empty, fmt.Errorf("open reference workbook: %w", err)
	}
	defer f.Close()
	return importReferenceWorkbook(db, f, filename, sheet)
}

func ImportReferenceRatesReader(db *sql.DB, reader io.Reader, filename, sheet string) (ReferenceRateImport, error) {
	f, err := excelize.OpenReader(reader, excelize.Options{UnzipSizeLimit: 64 << 20, UnzipXMLSizeLimit: 16 << 20})
	if err != nil {
		return ReferenceRateImport{}, fmt.Errorf("open reference workbook: %w", err)
	}
	defer f.Close()
	return importReferenceWorkbook(db, f, filename, sheet)
}

func importReferenceWorkbook(db *sql.DB, f *excelize.File, filename, sheet string) (ReferenceRateImport, error) {
	var empty ReferenceRateImport
	sheet = strings.TrimSpace(sheet)
	if sheet == "" {
		var matches []string
		for _, name := range f.GetSheetList() {
			iterator, err := f.Rows(name)
			if err != nil {
				return empty, err
			}
			var header []string
			if iterator.Next() {
				header, err = iterator.Columns(excelize.Options{RawCellValue: true})
			}
			rowErr := iterator.Error()
			closeErr := iterator.Close()
			if err != nil {
				return empty, err
			}
			if rowErr != nil {
				return empty, rowErr
			}
			if closeErr != nil {
				return empty, closeErr
			}
			if _, err := referenceRateColumns(header); err == nil {
				matches = append(matches, name)
			}
		}
		switch len(matches) {
		case 0:
			return empty, fmt.Errorf("no reference-rate worksheet found; expected Date followed by columns such as USD (INR / 1 USD) in the first row")
		case 1:
			sheet = matches[0]
		default:
			return empty, fmt.Errorf("multiple reference-rate worksheets found: %s; specify a worksheet override", strings.Join(matches, ", "))
		}
	}
	rows, err := f.GetRows(sheet, excelize.Options{RawCellValue: true})
	if err != nil {
		return empty, err
	}
	if len(rows) < 2 || len(rows[0]) < 2 || strings.TrimSpace(rows[0][0]) != "Date" {
		return empty, fmt.Errorf("expected Date followed by currency rate columns and at least one data row")
	}
	columns, err := referenceRateColumns(rows[0])
	if err != nil {
		return empty, err
	}
	props, err := f.GetWorkbookProps()
	if err != nil {
		return empty, err
	}
	use1904 := props.Date1904 != nil && *props.Date1904
	tx, err := db.Begin()
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	insert, err := tx.Prepare(`INSERT INTO reference_rates
 (rate_date,currency,units,rate_inr,source_file,source_sheet) VALUES(?,?,?,?,?,?)
 ON CONFLICT(rate_date,currency) DO NOTHING`)
	if err != nil {
		return empty, err
	}
	defer insert.Close()
	result := ReferenceRateImport{Sheet: sheet}
	for i, row := range rows[1:] {
		if strings.TrimSpace(strings.Join(row, "")) == "" {
			continue
		}
		if len(row) > len(columns)+1 {
			return empty, fmt.Errorf("row %d: expected a date and %d rates", i+2, len(columns))
		}
		date, err := referenceRateDate(strings.TrimSpace(row[0]), use1904)
		if err != nil {
			return empty, fmt.Errorf("row %d: %w", i+2, err)
		}
		if result.FromDate == "" || date < result.FromDate {
			result.FromDate = date
		}
		if date > result.ToDate {
			result.ToDate = date
		}
		for j, column := range columns {
			value := ""
			if j+1 < len(row) {
				value = strings.TrimSpace(row[j+1])
			}
			if value == "" {
				result.EmptySkipped++
				continue
			}
			rate, err := strconv.ParseFloat(value, 64)
			if err != nil || math.IsNaN(rate) || math.IsInf(rate, 0) || rate <= 0 {
				return empty, fmt.Errorf("row %d %s: rate must be a positive finite number", i+2, column.currency)
			}
			res, err := insert.Exec(date, column.currency, column.units, rate, filepath.Base(filename), sheet)
			if err != nil {
				return empty, err
			}
			n, err := res.RowsAffected()
			if err != nil {
				return empty, err
			}
			if n == 0 {
				var existingRate float64
				var existingUnits int64
				if err := tx.QueryRow(`SELECT units,rate_inr FROM reference_rates WHERE rate_date=? AND currency=?`, date, column.currency).Scan(&existingUnits, &existingRate); err != nil {
					return empty, err
				}
				if existingUnits != column.units || existingRate != rate {
					return empty, fmt.Errorf("row %d: conflicting reference rate for %s %s", i+2, date, column.currency)
				}
				result.Skipped++
			} else {
				result.Inserted++
			}
		}
	}
	if result.Inserted+result.Skipped+result.EmptySkipped == 0 {
		return empty, fmt.Errorf("workbook contains no reference rates")
	}
	if err := tx.Commit(); err != nil {
		return empty, err
	}
	return result, nil
}

type quoteColumn struct {
	currency string
	units    int64
}

func referenceRateColumns(headerRow []string) ([]quoteColumn, error) {
	if len(headerRow) < 2 || strings.TrimSpace(headerRow[0]) != "Date" {
		return nil, fmt.Errorf("expected Date followed by currency rate columns")
	}
	columns := make([]quoteColumn, 0, len(headerRow)-1)
	seen := map[string]bool{}
	for _, header := range headerRow[1:] {
		match := referenceRateHeader.FindStringSubmatch(strings.TrimSpace(header))
		if match == nil || match[1] != match[3] || match[1] == "INR" || seen[match[1]] {
			return nil, fmt.Errorf("invalid or duplicate rate header %q; expected USD (INR / 1 USD), for example", header)
		}
		units, err := strconv.ParseInt(match[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid units in %q", header)
		}
		seen[match[1]] = true
		columns = append(columns, quoteColumn{match[1], units})
	}

	return columns, nil
}

func referenceRateDate(value string, use1904 bool) (string, error) {
	for _, layout := range []string{"02/01/2006", "2006-01-02"} {
		if date, err := time.Parse(layout, value); err == nil {
			return date.Format("2006-01-02"), nil
		}
	}
	// Excel cells may contain serial dates even when displayed as calendar dates.
	if serial, err := strconv.ParseFloat(value, 64); err == nil && serial >= 1 && serial <= 2957003 && math.Trunc(serial) == serial {
		if date, err := excelize.ExcelDateToTime(serial, use1904); err == nil && date.Year() <= 9999 {
			return date.Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("invalid reference date %q", value)
}
