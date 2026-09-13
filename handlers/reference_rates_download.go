package handlers

import (
	"fmt"
	"github.com/xuri/excelize/v2"
	"net/http"
	"time"
)

func (s *Server) DownloadReferenceRates(w http.ResponseWriter, r *http.Request) {
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	start, e1 := time.Parse("2006-01-02", from)
	end, e2 := time.Parse("2006-01-02", to)
	if e1 != nil || e2 != nil || start.After(end) {
		http.Error(w, "Choose valid start and end dates, with start on or before end", 400)
		return
	}
	rows, err := s.DB.Query(`SELECT rate_date,currency,units,rate_inr FROM reference_rates WHERE rate_date>=? AND rate_date<=? ORDER BY rate_date,currency`, from, to)
	if err != nil {
		http.Error(w, "Could not load reference rates", 500)
		return
	}
	defer rows.Close()
	type quote struct {
		date, key string
		rate      float64
	}
	quotes := []quote{}
	columns := map[string]int{}
	header := []any{"Date"}
	for rows.Next() {
		var date, currency string
		var units int64
		var rate float64
		if err = rows.Scan(&date, &currency, &units, &rate); err != nil {
			http.Error(w, "Could not read rates", 500)
			return
		}
		key := fmt.Sprintf("%s (INR / %d %s)", currency, units, currency)
		if _, ok := columns[key]; !ok {
			columns[key] = len(header)
			header = append(header, key)
		}
		quotes = append(quotes, quote{date, key, rate})
	}
	if rows.Err() != nil {
		http.Error(w, "Could not read rates", 500)
		return
	}
	if len(quotes) == 0 {
		http.Error(w, "No reference rates found in the selected date range", 404)
		return
	}
	f := excelize.NewFile()
	defer f.Close()
	err = f.SetSheetRow("Sheet1", "A1", &header)
	rowNumber := 1
	current := ""
	var cells []any
	flush := func() {
		if cells != nil && err == nil {
			err = f.SetSheetRow("Sheet1", fmt.Sprintf("A%d", rowNumber), &cells)
		}
	}
	for _, q := range quotes {
		if q.date != current {
			flush()
			rowNumber++
			current = q.date
			cells = make([]any, len(header))
			cells[0] = q.date
		}
		cells[columns[q.key]] = q.rate
	}
	flush()
	last, _ := excelize.ColumnNumberToName(len(header))
	if err == nil {
		err = f.SetColWidth("Sheet1", "A", last, 25)
	}
	if err != nil {
		http.Error(w, "Could not create workbook", 500)
		return
	}
	buffer, err := f.WriteToBuffer()
	if err != nil {
		http.Error(w, "Could not create workbook", 500)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="ReferenceRates-%s-to-%s.xlsx"`, from, to))
	w.Write(buffer.Bytes())
}
