package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/selectq/selectq/core"
)

func TestBusinessPartnerAddressAPI(t *testing.T) {
	db, err := core.InitDB(filepath.Join(t.TempDir(), "address-api.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := NewServer(db)
	call := func(handler http.HandlerFunc, method string, id int, payload any) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(method, "/", strings.NewReader(string(body)))
		r = mux.SetURLVars(r, map[string]string{"id": fmt.Sprint(id)})
		w := httptest.NewRecorder()
		handler(w, r)
		return w
	}
	w := call(server.CreateBusinessPartner, "POST", 0, map[string]any{"name": "API address partner", "addresses": []map[string]any{{"address": "Original office"}, {"address": "Second office"}}})
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var bp core.BusinessPartner
	if err = json.Unmarshal(w.Body.Bytes(), &bp); err != nil {
		t.Fatal(err)
	}
	if len(bp.Addresses) != 2 || bp.Addresses[0].ID == 0 {
		t.Fatal("created API response lacks saved addresses")
	}
	original := bp.Addresses[0].ID
	inv := core.SalesInvoice{BusinessPartnerID: bp.ID, AddressID: &original, InvoiceDate: "2026-09-11", Currency: "INR"}
	w = call(server.CreateSalesInvoice, "POST", 0, inv)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err = json.Unmarshal(w.Body.Bytes(), &inv); err != nil {
		t.Fatal(err)
	}
	bp.Addresses[0].Address = "New office"
	w = call(server.UpdateBusinessPartner, "PUT", bp.ID, bp)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err = json.Unmarshal(w.Body.Bytes(), &bp); err != nil {
		t.Fatal(err)
	}
	if len(bp.Addresses) != 3 || !bp.Addresses[0].IsArchived {
		t.Fatal("address version response missing")
	}
	w = call(server.GetSalesInvoiceByID, "GET", inv.ID, nil)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var historical core.SalesInvoice
	if err = json.Unmarshal(w.Body.Bytes(), &historical); err != nil {
		t.Fatal(err)
	}
	if historical.BillingAddress != "Original office" || historical.AddressID == nil || *historical.AddressID != original {
		t.Fatal("invoice address history changed")
	}
	w = call(server.UpdateSalesInvoice, "PUT", inv.ID, historical)
	if w.Code != 200 {
		t.Fatal("retaining archived address:", w.Body.String())
	}
	w = call(server.CreateSalesInvoice, "POST", 0, inv)
	if w.Code != 400 {
		t.Fatal("new invoice with archived address was not rejected:", w.Code, w.Body.String())
	}
	active := bp.Addresses[2].ID
	inv.AddressID = &active
	w = call(server.CreateSalesInvoice, "POST", 0, inv)
	if w.Code != 201 {
		t.Fatal("new active address:", w.Body.String())
	}
}
