package core

import (
	"context"
	"fmt"
	"github.com/coregx/gxpdf"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParseBankPurchasePDF recognizes the HDFC remittance/GST advice layout.
// Extraction only creates a draft; saving and duplicate validation remain separate.
func ParseBankPurchasePDF(ctx context.Context, data []byte) (PurchaseInvoice, error) {
	var p PurchaseInvoice
	doc, err := gxpdf.OpenFromBytesWithContext(ctx, data)
	if err != nil {
		return p, fmt.Errorf("Could not read PDF. Upload a readable, unencrypted bank GST invoice")
	}
	defer doc.Close()
	text, err := doc.ExtractTextFromPage(1)
	if err != nil {
		return p, fmt.Errorf("Could not extract PDF text")
	}
	return parseBankPurchaseText(text)
}

func parseBankPurchaseText(text string) (PurchaseInvoice, error) {
	var p PurchaseInvoice
	capture := func(pattern string) string {
		m := regexp.MustCompile(pattern).FindStringSubmatch(text)
		if len(m) > 1 {
			return strings.TrimSpace(m[1])
		}
		return ""
	}
	p.InvoiceNumber = capture(`(?i)GST\s+Invoice\s+Number\s*:\s*([A-Z0-9/-]+)`)
	date := capture(`(?i)Issue\s+Date\s*:\s*(\d{2}-[A-Za-z]{3}-\d{4})`)
	d, err := time.Parse("02-Jan-2006", date)
	if err != nil {
		return p, fmt.Errorf("Issue Date was not found. Enter this invoice manually")
	}
	p.InvoiceDate = d.Format("2006-01-02")
	p.PartyGSTIN = strings.ToUpper(capture(`(?i)Bank\s+GSTIN\s+Registration\s+number\s*:\s*([0-9A-Z]{15})`))
	block := capture(`(?is)From\s*\(POP\)\s*:\s*(.*?)State/UT`)
	lines := strings.Split(strings.ReplaceAll(block, "\r", ""), "\n")
	if len(lines) > 1 {
		p.PartyName = strings.TrimSpace(lines[0])
		p.PartyAddress = strings.TrimSpace(strings.Join(lines[1:], "\n"))
	}
	if p.InvoiceNumber == "" || p.PartyGSTIN == "" || p.PartyName == "" || p.PartyAddress == "" {
		return p, fmt.Errorf("Bank invoice details were not recognized. Enter this invoice manually")
	}
	// This template prints twelve fixed-decimal values, sometimes without spaces
	// between adjacent columns: service, five rate/amount pairs, then total GST.
	table := capture(`(?is)Value\s+Of\s+Service(.*?)Dear\s+Customer`)
	amounts := regexp.MustCompile(`[0-9][0-9,]*\.[0-9]{2}`).FindAllString(table, -1)
	if len(amounts) != 12 {
		return p, fmt.Errorf("Could not reliably read the service and GST amounts. Enter this invoice manually")
	}
	values := make([]float64, len(amounts))
	for i, a := range amounts {
		values[i], err = strconv.ParseFloat(strings.ReplaceAll(a, ",", ""), 64)
		if err != nil {
			return p, err
		}
	}
	tax := money(values[2] + values[4] + values[6] + values[8] + values[10])
	if tax != money(values[11]) {
		return p, fmt.Errorf("Extracted GST amounts do not match the invoice total. Enter this invoice manually")
	}
	p.TotalAmount = money(values[0] + tax)
	if p.TotalAmount <= 0 {
		return p, fmt.Errorf("Invoice total must be positive")
	}
	return p, nil
}
