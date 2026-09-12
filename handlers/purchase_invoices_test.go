package handlers

import (
	"bytes"
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/selectq/selectq/core"
	"mime/multipart"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
)

func TestPurchaseInvoiceRegister(t *testing.T) {
	db, err := core.InitDB(filepath.Join(t.TempDir(), "purchases.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = core.StoreInDB(db, core.AccountMeta{AccountNo: "purchase-test"}, []core.BankTransaction{
		{Date: "2026-09-12", WithdrawalAmt: 118, AccountHead: "Software/Services"},
		{Date: "2026-09-12", WithdrawalAmt: 100, AccountHead: "Salary"},
		{Date: "2026-09-12", WithdrawalAmt: 100, AccountHead: "To Personal"},
		{Date: "2026-09-12", DepositAmt: 100, AccountHead: "Sales Invoice"},
		{Date: "2026-09-12", WithdrawalAmt: 100, AccountHead: "Travel", InvoiceNumber: "existing"},
	}, "purchases.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(db)
	call := func(method string, p core.PurchaseInvoice, file []byte) *httptest.ResponseRecorder {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		data, _ := json.Marshal(p)
		if err := writer.WriteField("invoice", string(data)); err != nil {
			t.Fatal(err)
		}
		if file != nil {
			part, err := writer.CreateFormFile("file", "invoice.pdf")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = part.Write(file); err != nil {
				t.Fatal(err)
			}
		}
		writer.Close()
		r := httptest.NewRequest(method, "/purchase-invoices", &body)
		r.Header.Set("Content-Type", writer.FormDataContentType())
		r = mux.SetURLVars(r, map[string]string{"id": strconv.Itoa(p.ID)})
		w := httptest.NewRecorder()
		s.SavePurchaseInvoice(w, r)
		return w
	}
	id := 1
	p := core.PurchaseInvoice{BankTransactionID: &id, InvoiceDate: "2026-09-12", PartyName: "Supplier", PartyAddress: "Street\nCity", TotalAmount: 118}
	pdf := []byte("%PDF-1.4\n invoice document\n%%EOF")
	for _, bad := range []int{2, 3, 4, 5, 999} {
		p.BankTransactionID = &bad
		if w := call("POST", p, pdf); w.Code < 400 {
			t.Fatal("ineligible", bad, w.Code)
		}
	}
	p.BankTransactionID = &id
	if w := call("POST", p, []byte("<script>bad</script>")); w.Code != 400 {
		t.Fatal("invalid upload", w.Code)
	}
	w := call("POST", p, pdf)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err = json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if w := call("POST", p, pdf); w.Code != 400 {
		t.Fatal("duplicate", w.Code)
	}
	p.ExternalURL = "https://example.com/invoices/12?download=1"
	p.PartyName = "Corrected supplier"
	p.InvoiceNumber = "SUP-12"
	if w := call("PUT", p, nil); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	list := httptest.NewRecorder()
	s.GetPurchaseInvoices(list, httptest.NewRequest("GET", "/", nil))
	var invoices []core.PurchaseInvoice
	if err = json.Unmarshal(list.Body.Bytes(), &invoices); err != nil {
		t.Fatal(err)
	}
	if len(invoices) != 1 || invoices[0].ExternalURL != p.ExternalURL || invoices[0].FileName != "invoice.pdf" || invoices[0].PartyName != "Corrected supplier" {
		t.Fatal(invoices)
	}
	download := httptest.NewRecorder()
	r := mux.SetURLVars(httptest.NewRequest("GET", "/", nil), map[string]string{"id": strconv.Itoa(p.ID)})
	s.DownloadPurchaseInvoice(download, r)
	if download.Code != 200 || !bytes.Equal(download.Body.Bytes(), pdf) || download.Header().Get("Content-Type") != "application/pdf" {
		t.Fatal(download.Code, download.Body.String())
	}
	txns, err := core.GetTransactions(db, 1)
	if err != nil {
		t.Fatal(err)
	}
	if txns[0].PurchaseInvoiceID == nil || *txns[0].PurchaseInvoiceID != p.ID {
		t.Fatal("missing transaction link")
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM invoice_allocations`).Scan(&count); err != nil || count != 0 {
		t.Fatal("purchase created sales allocations", err, count)
	}
	standalone := core.PurchaseInvoice{InvoiceDate: "2026-09-12", ExternalURL: "https://example.com/invoice.pdf", PartyName: "Unlinked", PartyAddress: "City", TotalAmount: 50}
	if w := call("POST", standalone, nil); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, invalid := range []string{"javascript:alert(1)", "file:///invoice.pdf", "/relative", "https://", "https://user:password@example.com/invoice"} {
		standalone.ExternalURL = invalid
		if w := call("POST", standalone, nil); w.Code != 400 {
			t.Fatal("invalid external URL", invalid, w.Code)
		}
	}
	standalone.ExternalURL = ""
	p.ExternalURL = ""
	if w := call("PUT", p, nil); w.Code != 200 {
		t.Fatal("clear external URL", w.Code, w.Body.String())
	}
	cleared, err := core.GetPurchaseInvoices(db)
	if err != nil {
		t.Fatal(err)
	}
	for _, invoice := range cleared {
		if invoice.ID == p.ID && (invoice.ExternalURL != "" || invoice.FileName != "invoice.pdf") {
			t.Fatal("clear URL must preserve upload", invoice)
		}
	}
	standalone.PartyGSTIN = "bad"
	if w := call("POST", standalone, nil); w.Code != 400 {
		t.Fatal("invalid GSTIN", w.Code)
	}
	standalone.PartyGSTIN = ""
	standalone.TotalAmount = 0
	if w := call("POST", standalone, nil); w.Code != 400 {
		t.Fatal("invalid amount", w.Code)
	}
}
