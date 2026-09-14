package core

import (
	"bytes"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestStatementDownload(t *testing.T) {
	dir := t.TempDir()
	db, err := InitDB(filepath.Join(dir, "statement.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	f.SetCellStr(sheet, "A1", "Original bank letterhead")
	f.SetCellStr(sheet, "A2", "Account No :12345")
	f.MergeCell(sheet, "A1", "G1")
	f.SetColWidth(sheet, "B", "B", 45)
	header := []interface{}{"Date", "Narration", "Chq./Ref.No.", "Value Dt", "Withdrawal Amt.", "Deposit Amt.", "Closing Balance"}
	f.SetSheetRow(sheet, "A21", &header)
	first := []interface{}{"01/09/2026", "Receipt", "0000123", "01/09/2026", 0, 100, 1000}
	second := []interface{}{"02/09/2026", "Purchase", "0000124", "02/09/2026", 20, 0, 980}
	f.SetSheetRow(sheet, "A23", &first)
	// The parser skips rows without narration; classifications must not drift.
	f.SetCellStr(sheet, "A24", "02/09/2026")
	f.SetSheetRow(sheet, "A25", &second)
	f.SetCellStr(sheet, "A26", "*** End of statement ***")
	f.SetCellStr(sheet, "H28", "Original footer at right")
	f.NewSheet("Notes")
	f.SetCellStr("Notes", "A1", "Preserve this sheet")
	buffer, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	source := append([]byte(nil), buffer.Bytes()...)
	path := filepath.Join(dir, "bank.xlsx")
	if err := os.WriteFile(path, source, 0600); err != nil {
		t.Fatal(err)
	}
	meta, txns, err := ParseStatement(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := StoreInDB(db, meta, txns, "bank.xlsx", source); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO business_partners(id,name) VALUES(1,'Studio South Design');
		UPDATE bank_transactions SET account_head='Sales Invoice',sub_account_head='=literal',invoice_number='001/2026-2027, 002/2026-2027',business_partner_id=1 WHERE id=1;
		UPDATE bank_transactions SET account_head='Expense',sub_account_head='Software' WHERE id=2;
		INSERT INTO purchase_invoices(bank_transaction_id,invoice_number,invoice_date,party_name,party_address,total_amount) VALUES(2,'PUR-12','2026-09-02','Vendor','Address',20)`); err != nil {
		t.Fatal(err)
	}
	check := func(data []byte) {
		t.Helper()
		out, err := excelize.OpenReader(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		defer out.Close()
		for cell, want := range map[string]string{
			"A1": "Original bank letterhead", "C23": "0000123", "H21": "Account Head", "I21": "Sub Account Head", "J21": "Invoice",
			"H23": "Sales Invoice", "I23": "Studio South Design", "J23": "001/2026-2027, 002/2026-2027", "H24": "", "H25": "Expense", "J25": "PUR-12",
			"A26": "*** End of statement ***", "K28": "Original footer at right",
		} {
			got, err := out.GetCellValue(sheet, cell)
			if err != nil || got != want {
				t.Errorf("%s: %q, want %q (%v)", cell, got, want, err)
			}
		}
		if formula, _ := out.GetCellFormula(sheet, "I23"); formula != "" {
			t.Fatal("classification became a formula")
		}
		if width, _ := out.GetColWidth(sheet, "B"); width != 45 {
			t.Fatal("original width changed")
		}
		if value, _ := out.GetCellValue("Notes", "A1"); value != "Preserve this sheet" {
			t.Fatal("lost extra sheet")
		}
		if merges, _ := out.GetMergeCells(sheet); len(merges) != 1 || merges[0].GetEndAxis() != "G1" {
			t.Fatal("lost letterhead merge")
		}
	}
	data, filename, err := StatementDownload(db, 1, dir)
	if err != nil {
		t.Fatal(err)
	}
	if filename != "bank-classified.xlsx" {
		t.Fatal(filename)
	}
	check(data)
	var retained []byte
	if err := db.QueryRow(`SELECT workbook FROM statement_workbooks WHERE import_id=1`).Scan(&retained); err != nil || !bytes.Equal(source, retained) {
		t.Fatal("original changed", err)
	}
	if _, err := db.Exec(`DELETE FROM statement_workbooks`); err != nil {
		t.Fatal(err)
	}
	data, _, err = StatementDownload(db, 1, dir)
	if err != nil {
		t.Fatal(err)
	}
	check(data)
	if _, _, err = StatementDownload(db, 1, t.TempDir()); !errors.Is(err, ErrStatementOriginalUnavailable) {
		t.Fatal(err)
	}
	if _, _, err = StatementDownload(db, 999, dir); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE bank_transactions SET deposit_amt=101 WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if _, _, err = StatementDownload(db, 1, dir); !errors.Is(err, ErrStatementMismatch) {
		t.Fatal(err)
	}
	unchanged, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(source, unchanged) {
		t.Fatal("original disk file changed")
	}
}
