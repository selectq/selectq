package core

import (
	"context"
	"os"
	"testing"
)

func TestBankPurchasePDFSample(t *testing.T) {
	data, err := os.ReadFile("../060826I049902319.pdf")
	if os.IsNotExist(err) {
		t.Skip("Local sample PDF is not available")
	}
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseBankPurchasePDF(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	if p.InvoiceNumber != "DPO2721818165521" || p.InvoiceDate != "2026-08-06" || p.PartyGSTIN != "06AAACH2702H1Z4" || p.PartyName != "HDFC Bank Ltd" || p.TotalAmount != 2297.45 || p.PartyAddress == "" {
		t.Fatalf("incorrect extraction: %+v", p)
	}
}
func TestBankPurchaseTextValidation(t *testing.T) {
	text := `Value Of Service
IGST rate Amount CGST rate Amount SGST rate Amount UTGST rate Amount Cess rate Amount Grand Total
1946.990.00 0.00 9.00175.23 9.00 175.23 0.00 0.00 0.00 0.00 350.46
Dear Customer,
GST Invoice Number : BANK-001
Issue Date : 06-Aug-2026
From (POP):
Example Bank
Branch address
State/UT name: Haryana
Customer GSTIN Registration number : 06AKLPM9722C2Z3
Bank GSTIN Registration number : 06AAACH2702H1Z4`
	p, err := parseBankPurchaseText(text)
	if err != nil || p.TotalAmount != 2297.45 || p.PartyGSTIN != "06AAACH2702H1Z4" {
		t.Fatal(p, err)
	}
	if _, err = parseBankPurchaseText("Scanned or unsupported invoice"); err == nil {
		t.Fatal("unsupported accepted")
	}
	if _, err = ParseBankPurchasePDF(context.Background(), []byte("not a pdf")); err == nil {
		t.Fatal("invalid PDF accepted")
	}
}
