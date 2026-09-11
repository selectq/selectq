package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/selectq/selectq/core"
	"github.com/xuri/excelize/v2"
	"net/http"
)

func (s *Server) GetGSTR3B(w http.ResponseWriter, r *http.Request)      { s.gstr3b(w, r, false) }
func (s *Server) DownloadGSTR3B(w http.ResponseWriter, r *http.Request) { s.gstr3b(w, r, true) }

func (s *Server) gstr3b(w http.ResponseWriter, r *http.Request, download bool) {
	fy := r.URL.Query().Get("financial_year")
	if (download || fy != "") && !core.ValidFinancialYear(fy) {
		http.Error(w, "Select a financial year in YYYY-YYYY format (April–March)", 400)
		return
	}
	summary, err := core.GetGSTR3BSummary(s.DB, fy)
	if err != nil {
		http.Error(w, "Could not load GSTR-3B summary", 500)
		return
	}
	if !download {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summary)
		return
	}
	f := excelize.NewFile()
	defer f.Close()
	sheet := "Sheet1"
	header := []any{"Invoice Number", "Currency", "Amount in Currency", "Invoiced To", "Address", "Invoice Date", "Exchange Rate (INR per unit)", "Value in INR", "Reference Date", "Rate Status"}
	if err = f.SetSheetRow(sheet, "A1", &header); err != nil {
		http.Error(w, "Could not create workbook", 500)
		return
	}
	for i, row := range summary.Rows {
		var rate, value, date any
		status := "Missing reference rate"
		if row.ExchangeRate != nil {
			rate = *row.ExchangeRate
			value = *row.ValueINR
			status = "Available"
		}
		if row.ReferenceDate != nil {
			date = *row.ReferenceDate
		}
		if row.Currency == "INR" {
			status = "INR (rate 1)"
			date = "Not applicable"
		}
		cells := []any{row.InvoiceNumber, row.Currency, row.Amount, row.InvoicedTo, row.Address, row.InvoiceDate, rate, value, date, status}
		if err = f.SetSheetRow(sheet, fmt.Sprintf("A%d", i+2), &cells); err != nil {
			http.Error(w, "Could not create workbook", 500)
			return
		}
	}
	if err = f.SetColWidth(sheet, "A", "J", 24); err != nil {
		http.Error(w, "Could not format workbook", 500)
		return
	}
	addressStyle, err := f.NewStyle(&excelize.Style{Alignment: &excelize.Alignment{WrapText: true}})
	if err != nil {
		http.Error(w, "Could not format workbook", 500)
		return
	}
	if len(summary.Rows) > 0 {
		if err = f.SetCellStyle(sheet, "E2", fmt.Sprintf("E%d", len(summary.Rows)+1), addressStyle); err != nil {
			http.Error(w, "Could not format workbook", 500)
			return
		}
	}
	buffer, err := f.WriteToBuffer()
	if err != nil {
		http.Error(w, "Could not create workbook", 500)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="GSTR3B-%s.xlsx"`, fy))
	w.Write(buffer.Bytes())
}
