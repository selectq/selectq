package core

import (
	"github.com/xuri/excelize/v2"
	"path/filepath"
	"testing"
)

func rateWorkbook(t *testing.T, rows [][]any) string {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	for i, row := range rows {
		cell, err := excelize.CoordinatesToCellName(1, i+1)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.SetSheetRow("Sheet1", cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "rates.xlsx")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReferenceRatesUnitsAndRepeatImport(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "rates.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	path := rateWorkbook(t, [][]any{
		{"Date", "USD (INR / 1 USD)", "JPY (INR / 100 JPY)", "IDR (INR / 10000 IDR)"},
		{"31/08/2026", "95.4509", "59.7200", "53.7816"},
		{"02/04/2026", "93.2088", "58.4900", "54.7646"},
	})
	result, err := ImportReferenceRates(db, path, "Sheet1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Inserted != 6 || result.Skipped != 0 || result.FromDate != "2026-04-02" || result.ToDate != "2026-08-31" {
		t.Fatalf("import: %+v", result)
	}
	for _, tc := range []struct {
		currency string
		units    int
		rate     float64
	}{
		{"USD", 1, 95.4509}, {"JPY", 100, 59.72}, {"IDR", 10000, 53.7816},
	} {
		var units int
		var rate float64
		var file, sheet string
		err := db.QueryRow(`SELECT units,rate_inr,source_file,source_sheet FROM reference_rates WHERE rate_date=? AND currency=?`, "2026-08-31", tc.currency).Scan(&units, &rate, &file, &sheet)
		if err != nil {
			t.Fatal(err)
		}
		if units != tc.units || rate != tc.rate || file != "rates.xlsx" || sheet != "Sheet1" {
			t.Fatalf("%s: %d %v %s %s", tc.currency, units, rate, file, sheet)
		}
	}
	if err := initReferenceRates(db); err != nil {
		t.Fatal(err)
	}
	repeat, err := ImportReferenceRates(db, path, "Sheet1")
	if err != nil || repeat.Inserted != 0 || repeat.Skipped != 6 {
		t.Fatalf("repeat %+v: %v", repeat, err)
	}
}

func TestReferenceRateImportRollsBackInvalidRows(t *testing.T) {
	for _, invalid := range [][]any{
		{"03/04/2026", "NaN"}, {"03/04/2026", "Inf"}, {"03/04/2026", -1},
		{"03/04/2026", 0}, {"03/04/2026", "N/A"}, {"31/02/2026", 95},
		{"03/04/2026", 95, "unexpected column"},
	} {
		db, err := InitDB(filepath.Join(t.TempDir(), "rates.db"))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		path := rateWorkbook(t, [][]any{{"Date", "USD (INR / 1 USD)"}, {"02/04/2026", 93.2088}, invalid})
		if _, err := ImportReferenceRates(db, path, "Sheet1"); err == nil {
			t.Fatalf("accepted invalid row: %v", invalid)
		}
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM reference_rates`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("partial import: %d", count)
		}
	}
}

func TestReferenceRateConflictPreservesExistingData(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "rates.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	path := rateWorkbook(t, [][]any{{"Date", "USD (INR / 1 USD)"}, {"02/04/2026", 93.2088}})
	if _, err := ImportReferenceRates(db, path, "Sheet1"); err != nil {
		t.Fatal(err)
	}
	conflict := rateWorkbook(t, [][]any{{"Date", "USD (INR / 1 USD)"}, {"03/04/2026", 95}, {"02/04/2026", 96}})
	if _, err := ImportReferenceRates(db, conflict, "Sheet1"); err == nil {
		t.Fatal("accepted conflicting rate")
	}
	var count int
	var rate float64
	if err := db.QueryRow(`SELECT COUNT(*),MAX(rate_inr) FROM reference_rates`).Scan(&count, &rate); err != nil {
		t.Fatal(err)
	}
	if count != 1 || rate != 93.2088 {
		t.Fatalf("conflict changed data: %d %v", count, rate)
	}
}

func TestReferenceRateDatesAndHeaders(t *testing.T) {
	for _, tc := range []struct {
		value   string
		use1904 bool
		want    string
	}{
		{"02/04/2026", false, "2026-04-02"}, {"2026-08-31", false, "2026-08-31"},
		{"45000", false, "2023-03-15"}, {"45000", true, "2027-03-16"},
	} {
		got, err := referenceRateDate(tc.value, tc.use1904)
		if err != nil || got != tc.want {
			t.Fatalf("date %+v: %s %v", tc, got, err)
		}
	}
	db, err := InitDB(filepath.Join(t.TempDir(), "rates.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, header := range []string{"USD", "USD (INR / 0 USD)", "USD (INR / 1 EUR)", "INR (INR / 1 INR)"} {
		path := rateWorkbook(t, [][]any{{"Date", header}, {"02/04/2026", 95}})
		if _, err := ImportReferenceRates(db, path, "Sheet1"); err == nil {
			t.Fatalf("accepted header %s", header)
		}
	}
}

func TestReferenceRatesSkipEmptyCells(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// A missing quote must not erase an already saved rate.
	if _, err := SaveReferenceRate(db, ReferenceRate{RateDate: "2026-08-31", Currency: "USD", Units: 1, RateINR: 95}, nil); err != nil {
		t.Fatal(err)
	}
	path := rateWorkbook(t, [][]any{
		{"Date", "USD (INR / 1 USD)", "JPY (INR / 100 JPY)", "IDR (INR / 10000 IDR)"},
		{"31/08/2026", nil, 59.72, nil},
		{"28/08/2026", 95.5614},
		{"27/08/2026"},
		{},
		{"26/08/2026", "  ", nil, 53.78},
	})
	for i := 0; i < 2; i++ {
		result, err := ImportReferenceRates(db, path, "")
		if err != nil {
			t.Fatal(err)
		}
		if result.Inserted != 3*(1-i) || result.Skipped != 3*i || result.EmptySkipped != 9 || result.Sheet != "Sheet1" {
			t.Fatalf("sparse import %d: %+v", i, result)
		}
	}
	rates, err := GetReferenceRates(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(rates) != 4 {
		t.Fatalf("unexpected rates: %+v", rates)
	}
	var preserved float64
	if err := db.QueryRow(`SELECT rate_inr FROM reference_rates WHERE rate_date='2026-08-31' AND currency='USD'`).Scan(&preserved); err != nil || preserved != 95 {
		t.Fatalf("existing quote changed: %v %v", preserved, err)
	}
	blank := rateWorkbook(t, [][]any{{"Date", "USD (INR / 1 USD)"}, {"01/09/2026"}})
	result, err := ImportReferenceRates(db, blank, "")
	if err != nil || result.Inserted != 0 || result.EmptySkipped != 1 {
		t.Fatalf("all blank quotes: %+v %v", result, err)
	}
	invalid := rateWorkbook(t, [][]any{
		{"Date", "USD (INR / 1 USD)", "JPY (INR / 100 JPY)"},
		{"01/09/2026", nil, 60},
		{"02/09/2026", 95, "not a rate"},
	})
	if _, err := ImportReferenceRates(db, invalid, ""); err == nil {
		t.Fatal("accepted nonempty invalid rate")
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM reference_rates`).Scan(&count); err != nil || count != 4 {
		t.Fatalf("partial import: %d %v", count, err)
	}
}

func TestReferenceRateWorksheetDiscovery(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "discovery.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetCellValue("Sheet1", "A1", "Workbook cover"); err != nil {
		t.Fatal(err)
	}
	addSheet := func(name, date string) {
		t.Helper()
		if _, err := f.NewSheet(name); err != nil {
			t.Fatal(err)
		}
		header := []any{"Date", "JPY (INR / 100 JPY)"}
		row := []any{date, 59.72}
		if err := f.SetSheetRow(name, "A1", &header); err != nil {
			t.Fatal(err)
		}
		if err := f.SetSheetRow(name, "A2", &row); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "workbook.xlsx")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportReferenceRates(db, path, ""); err == nil {
		t.Fatal("accepted workbook without rate headers")
	}
	addSheet("Renamed August Rates", "31/08/2026")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	result, err := ImportReferenceRates(db, path, "")
	if err != nil || result.Sheet != "Renamed August Rates" || result.Inserted != 1 {
		t.Fatalf("discovery %+v %v", result, err)
	}
	rates, err := GetReferenceRates(db)
	if err != nil || len(rates) != 1 || rates[0].SourceSheet != result.Sheet {
		t.Fatalf("provenance %+v %v", rates, err)
	}
	addSheet("Second Rates", "28/08/2026")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportReferenceRates(db, path, ""); err == nil {
		t.Fatal("silently selected one of multiple matching worksheets")
	}
	rates, err = GetReferenceRates(db)
	if err != nil || len(rates) != 1 {
		t.Fatalf("ambiguous import changed data: %+v %v", rates, err)
	}
	result, err = ImportReferenceRates(db, path, "Second Rates")
	if err != nil || result.Inserted != 1 || result.Sheet != "Second Rates" {
		t.Fatalf("override %+v %v", result, err)
	}
}
