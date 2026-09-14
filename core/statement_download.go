package core

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

var ErrStatementOriginalUnavailable = errors.New("The original statement workbook is unavailable. Restore the original file to the application folder to download it with classifications.")
var ErrStatementMismatch = errors.New("The original workbook does not match the imported transactions.")

// StatementDownload annotates a copy of the original workbook. Legacy imports
// can use a matching original file from the application folder.
func StatementDownload(db *sql.DB, importID int, originalDir string) ([]byte, string, error) {
	var filename, account string
	var source []byte
	err := db.QueryRow(`SELECT i.source_file, i.account_no, w.workbook FROM imports i
		LEFT JOIN statement_workbooks w ON w.import_id=i.id WHERE i.id=?`, importID).Scan(&filename, &account, &source)
	if err != nil {
		return nil, "", err
	}
	if len(source) == 0 {
		// Only a plain filename from the import may be resolved in this folder.
		if filename != filepath.Base(filename) || strings.ContainsAny(filename, `/\:`) || !strings.EqualFold(filepath.Ext(filename), ".xlsx") {
			return nil, "", ErrStatementOriginalUnavailable
		}
		source, err = os.ReadFile(filepath.Join(originalDir, filename))
		if err != nil {
			return nil, "", ErrStatementOriginalUnavailable
		}
	}
	f, err := excelize.OpenReader(bytes.NewReader(source))
	if err != nil {
		return nil, "", fmt.Errorf("read original workbook: %w", err)
	}
	defer f.Close()
	meta, original, err := parseStatementWorkbook(f)
	if err != nil {
		return nil, "", err
	}
	txns, err := GetTransactions(db, importID)
	if err != nil {
		return nil, "", err
	}
	if meta.AccountNo != account || len(original) != len(txns) {
		return nil, "", ErrStatementMismatch
	}
	for i, txn := range txns {
		if txn.PurchaseInvoiceID != nil {
			if err := db.QueryRow(`SELECT invoice_number FROM purchase_invoices WHERE id=?`, *txn.PurchaseInvoiceID).Scan(&txns[i].InvoiceNumber); err != nil {
				return nil, "", err
			}
		} else if strings.EqualFold(strings.TrimSpace(txn.AccountHead), "Sales Invoice") || len(txn.Allocations) > 0 {
			partnerName := txn.BusinessPartnerName
			if partnerName == "" && len(txn.Allocations) > 0 {
				if err := db.QueryRow(`SELECT COALESCE(group_concat(name, ', '), '') FROM (
					SELECT DISTINCT bp.name FROM invoice_allocations a
					JOIN sales_invoices s ON s.id=a.sales_invoice_id
					JOIN business_partners bp ON bp.id=s.business_partner_id
					WHERE a.bank_transaction_id=? ORDER BY bp.name)`, txn.ID).Scan(&partnerName); err != nil {
					return nil, "", err
				}
			}
			if partnerName != "" {
				txns[i].SubAccountHead = partnerName
			}
		}
		o := original[i]
		if o.Date != txn.Date || o.Narration != txn.Narration || o.ChqRefNo != txn.ChqRefNo || o.ValueDate != txn.ValueDate ||
			o.WithdrawalAmt != txn.WithdrawalAmt || o.DepositAmt != txn.DepositAmt || o.ClosingBalance != txn.ClosingBalance {
			return nil, "", ErrStatementMismatch
		}
	}
	sheet := f.GetSheetName(0)
	// Insert columns after the seven bank columns, preserving any existing cells
	// farther right (including header metadata and footer information).
	if err := f.InsertCols(sheet, "H", 3); err != nil {
		return nil, "", err
	}
	style, err := f.NewStyle(&excelize.Style{Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"}})
	if err != nil {
		return nil, "", err
	}
	for offset, label := range []string{"Account Head", "Sub Account Head", "Invoice"} {
		cell, _ := excelize.CoordinatesToCellName(8+offset, headerRow)
		if err := f.SetCellStr(sheet, cell, label); err != nil {
			return nil, "", err
		}
		headerStyle, err := f.GetCellStyle(sheet, fmt.Sprintf("G%d", headerRow))
		if err != nil {
			return nil, "", err
		}
		if err := f.SetCellStyle(sheet, cell, cell, headerStyle); err != nil {
			return nil, "", err
		}
	}
	if err := f.SetColWidth(sheet, "H", "J", 26); err != nil {
		return nil, "", err
	}
	for i, txn := range txns {
		for offset, value := range []string{txn.AccountHead, txn.SubAccountHead, txn.InvoiceNumber} {
			cell, _ := excelize.CoordinatesToCellName(8+offset, original[i].SourceRow)
			// Explicit strings keep invoice references and formula-like labels as text.
			if err := f.SetCellStr(sheet, cell, value); err != nil {
				return nil, "", err
			}
			if err := f.SetCellStyle(sheet, cell, cell, style); err != nil {
				return nil, "", err
			}
		}
	}
	output, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}
	return output.Bytes(), strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)) + "-classified.xlsx", nil
}
