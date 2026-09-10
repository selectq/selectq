package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/selectq/selectq/core"
	"github.com/gorilla/mux"
)

type Server struct {
	DB *sql.DB
}

func NewServer(db *sql.DB) *Server {
	return &Server{DB: db}
}

func (s *Server) GetImports(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	imports, err := core.GetImports(s.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(imports)
}

func (s *Server) GetTransactions(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	importID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid import ID", http.StatusBadRequest)
		return
	}
	txns, err := core.GetTransactions(s.DB, importID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(txns)
}

// TransactionUpdatePayload defines what fields can be updated
type TransactionUpdatePayload struct {
	AccountHead       string `json:"account_head"`
	SubAccountHead    string `json:"sub_account_head"`
	InvoiceNumber     string `json:"invoice_number"`
	BusinessPartnerID *int   `json:"business_partner_id"`
}

func (s *Server) UpdateTransaction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid transaction ID", http.StatusBadRequest)
		return
	}

	var payload TransactionUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := core.UpdateTransactionClassification(s.DB, id, payload.AccountHead, payload.SubAccountHead, payload.InvoiceNumber, payload.BusinessPartnerID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "Transaction updated successfully"})
}

func (s *Server) GetBusinessPartners(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	query := r.URL.Query().Get("q")
	partners, err := core.GetBusinessPartners(s.DB, query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(partners)
}

func (s *Server) CreateBusinessPartner(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var p core.BusinessPartner
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id, err := core.CreateBusinessPartner(s.DB, p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	p.ID = int(id)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p)
}

func (s *Server) GetBusinessPartnerByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	p, err := core.GetBusinessPartnerByID(s.DB, id)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(p)
}

func (s *Server) UpdateBusinessPartner(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var p core.BusinessPartner
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p.ID = id
	if err := core.UpdateBusinessPartner(s.DB, p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(p)
}

func (s *Server) DeleteBusinessPartner(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := core.DeleteBusinessPartner(s.DB, id); err != nil {
		// Check if it's a referential integrity error
		if len(err.Error()) > 14 && err.Error()[:14] == "Cannot delete:" {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"message": "Business partner deleted successfully"})
}

func (s *Server) UploadStatement(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB limit
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error retrieving the file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Save uploaded file to temp file
	tempFile, err := os.CreateTemp("", "upload-*.xlsx")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile.Name()) // Clean up
	defer tempFile.Close()

	if _, err := io.Copy(tempFile, file); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tempFile.Close() // Close before parsing

	// Parse the statement
	meta, transactions, err := core.ParseStatement(tempFile.Name())
	if err != nil {
		http.Error(w, fmt.Sprintf("Error parsing statement: %v", err), http.StatusBadRequest)
		return
	}

	// Classify transactions
	classified := 0
	for i := range transactions {
		core.ClassifyTransaction(&transactions[i])
		if transactions[i].AccountHead != "" {
			classified++
		}
	}

	// Store in database
	count, err := core.StoreInDB(s.DB, meta, transactions, handler.Filename)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error storing in database: %v", err), http.StatusInternalServerError)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    fmt.Sprintf("Successfully inserted %d transactions.", count),
		"classified": classified,
		"total":      len(transactions),
		"account":    meta.AccountNo,
		"period":     fmt.Sprintf("%s to %s", meta.StatementFrom, meta.StatementTo),
	})
}

func (s *Server) GetSalesInvoices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	invoices, err := core.GetSalesInvoices(s.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(invoices)
}

func (s *Server) GetSalesInvoiceByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	inv, err := core.GetSalesInvoiceByID(s.DB, id)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(inv)
}

func (s *Server) CreateSalesInvoice(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var inv core.SalesInvoice
	if err := json.NewDecoder(r.Body).Decode(&inv); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Calculate DueDate based on InvoiceDate and DueInDays
	if inv.DueInDays > 0 && inv.InvoiceDate != "" {
		dueDate, err := core.AddDaysToDate(inv.InvoiceDate, inv.DueInDays)
		if err == nil {
			inv.DueDate = dueDate
		}
	} else if inv.DueDate == "" {
		inv.DueDate = inv.InvoiceDate
	}

	id, err := core.CreateSalesInvoice(s.DB, inv)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return the full invoice with computed total and line items
	created, err := core.GetSalesInvoiceByID(s.DB, id)
	if err != nil {
		inv.ID = id
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(inv)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

func (s *Server) UpdateSalesInvoice(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var inv core.SalesInvoice
	if err := json.NewDecoder(r.Body).Decode(&inv); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	inv.ID = id

	// Calculate DueDate based on InvoiceDate and DueInDays
	if inv.DueInDays > 0 && inv.InvoiceDate != "" {
		dueDate, err := core.AddDaysToDate(inv.InvoiceDate, inv.DueInDays)
		if err == nil {
			inv.DueDate = dueDate
		}
	} else if inv.DueDate == "" {
		inv.DueDate = inv.InvoiceDate
	}

	if err := core.UpdateSalesInvoice(s.DB, inv); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return updated invoice
	updated, err := core.GetSalesInvoiceByID(s.DB, id)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"message": "Sales invoice updated successfully"})
		return
	}
	json.NewEncoder(w).Encode(updated)
}

func (s *Server) GetCompanyProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	profile, err := core.GetCompanyProfile(s.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(profile)
}

func (s *Server) SaveCompanyProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var profile core.CompanyProfile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := core.SaveCompanyProfile(s.DB, profile); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(profile)
}
