package core

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// parseStatement reads the HDFC statement XLSX and returns account metadata
// and a slice of transactions.
func ParseStatement(xlsxPath string) (AccountMeta, []BankTransaction, error) {
	f, err := excelize.OpenFile(xlsxPath)
	if err != nil {
		return AccountMeta{}, nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return AccountMeta{}, nil, fmt.Errorf("read rows: %w", err)
	}

	// --- Extract account metadata from header rows ---
	meta := extractMeta(rows)

	// --- Parse transaction rows ---
	var transactions []BankTransaction

	for i := DataStartRow - 1; i < len(rows); i++ { // DataStartRow is 1-indexed, rows is 0-indexed
		row := rows[i]

		// Stop conditions: empty row, asterisk separator, or footer text
		if len(row) == 0 {
			break
		}
		firstCell := strings.TrimSpace(safeGet(row, 0))
		if firstCell == "" || strings.HasPrefix(firstCell, "***") {
			break
		}

		txn := BankTransaction{
			Date:           strings.TrimSpace(safeGet(row, 0)),
			Narration:      strings.TrimSpace(safeGet(row, 1)),
			ChqRefNo:       strings.TrimSpace(safeGet(row, 2)),
			ValueDate:      strings.TrimSpace(safeGet(row, 3)),
			WithdrawalAmt:  parseAmount(safeGet(row, 4)),
			DepositAmt:     parseAmount(safeGet(row, 5)),
			ClosingBalance: parseAmount(safeGet(row, 6)),
		}

		// Validate: must have at least a date and narration
		if txn.Date == "" || txn.Narration == "" {
			continue
		}

		transactions = append(transactions, txn)
	}

	return meta, transactions, nil
}

// extractMeta pulls account info from the header section of the HDFC statement.
func extractMeta(rows [][]string) AccountMeta {
	meta := AccountMeta{}

	for i := 0; i < min(20, len(rows)); i++ {
		for _, cell := range rows[i] {
			cell = strings.TrimSpace(cell)
			switch {
			case strings.HasPrefix(cell, "Account No :"):
				// "Account No :50200020525472   Imperia"
				parts := strings.TrimPrefix(cell, "Account No :")
				meta.AccountNo = strings.Fields(parts)[0]
			case strings.HasPrefix(cell, "Cust ID :"):
				meta.CustomerID = strings.TrimPrefix(cell, "Cust ID :")
			case strings.HasPrefix(cell, "Account Branch :"):
				meta.Branch = strings.TrimPrefix(cell, "Account Branch :")
			case strings.HasPrefix(cell, "RTGS/NEFT IFSC :"):
				// "RTGS/NEFT IFSC :HDFC0004131   MICR :110240381"
				parts := strings.TrimPrefix(cell, "RTGS/NEFT IFSC :")
				tokens := strings.Split(parts, "MICR :")
				meta.IFSC = strings.TrimSpace(tokens[0])
				if len(tokens) > 1 {
					meta.MICR = strings.TrimSpace(tokens[1])
				}
			case strings.HasPrefix(cell, "Statement From"):
				// "Statement From  :  01/07/2026         To  :  31/08/2026"
				s := strings.TrimPrefix(cell, "Statement From")
				s = strings.TrimLeft(s, " :")
				toParts := strings.Split(s, "To")
				if len(toParts) >= 2 {
					meta.StatementFrom = strings.TrimSpace(strings.TrimLeft(toParts[0], " :"))
					meta.StatementTo = strings.TrimSpace(strings.TrimLeft(toParts[1], " :"))
				}
			}
		}
	}

	return meta
}

// --- Helpers ---

// safeGet returns the value at index i in a string slice, or "" if out of bounds.
func safeGet(row []string, i int) string {
	if i < len(row) {
		return row[i]
	}
	return ""
}

// parseAmount converts a cell value to float64. Returns 0 for empty/invalid values.
func parseAmount(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Remove commas (e.g., "1,23,456.78" Indian format)
	s = strings.ReplaceAll(s, ",", "")
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return val
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
