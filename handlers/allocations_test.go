package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
	"github.com/selectq/selectq/core"
)

func TestInvalidAllocationReturnsBadRequest(t *testing.T) {
	db, err := core.InitDB(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	req := httptest.NewRequest(http.MethodPut, "/api/transactions/999", bytes.NewBufferString(`{"account_head":"Sales Invoice","business_partner_id":1,"allocations":[{"sales_invoice_id":1,"amount":10}]}`))
	req = mux.SetURLVars(req, map[string]string{"id": "999"})
	response := httptest.NewRecorder()
	NewServer(db).UpdateTransaction(response, req)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
}
