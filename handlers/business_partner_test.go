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

func TestBusinessPartnerOptionalContactsAPI(t *testing.T) {
	db, err := core.InitDB(filepath.Join(t.TempDir(), "partners-api.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := NewServer(db)
	for i, contacts := range []string{"", `,"contacts":null`, `,"contacts":[]`} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			body := fmt.Sprintf(`{"name":"Partner %d","invoice_currency":"INR"%s}`, i, contacts)
			response := httptest.NewRecorder()
			server.CreateBusinessPartner(response, httptest.NewRequest(http.MethodPost, "/api/business-partners", strings.NewReader(body)))
			if response.Code != http.StatusCreated {
				t.Fatalf("create: %d %s", response.Code, response.Body.String())
			}
			var created core.BusinessPartner
			if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
				t.Fatal(err)
			}
			// Add a contact, then prove a no-contact update removes it.
			created.Contacts = []core.BusinessPartnerContact{{Name: "Optional"}}
			if err := core.UpdateBusinessPartner(db, created); err != nil {
				t.Fatal(err)
			}
			response = httptest.NewRecorder()
			request := mux.SetURLVars(httptest.NewRequest(http.MethodPut, "/api/business-partners/1", strings.NewReader(body)), map[string]string{"id": fmt.Sprint(created.ID)})
			server.UpdateBusinessPartner(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("update: %d %s", response.Code, response.Body.String())
			}
			response = httptest.NewRecorder()
			server.GetBusinessPartnerByID(response, mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/api/business-partners/1", nil), map[string]string{"id": fmt.Sprint(created.ID)}))
			if response.Code != http.StatusOK {
				t.Fatalf("get: %s", response.Body.String())
			}
			var saved core.BusinessPartner
			if err := json.Unmarshal(response.Body.Bytes(), &saved); err != nil {
				t.Fatal(err)
			}
			if saved.Contacts == nil || len(saved.Contacts) != 0 {
				t.Fatalf("expected contacts: [], got %s", response.Body.String())
			}
		})
	}
}
