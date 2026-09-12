package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/gorilla/mux"
	"github.com/selectq/selectq/core"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

func (s *Server) GetPurchaseInvoices(w http.ResponseWriter, r *http.Request) {
	invoices, err := core.GetPurchaseInvoices(s.DB)
	if err != nil {
		http.Error(w, "Could not load purchase invoices", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(invoices)
}
func (s *Server) SavePurchaseInvoice(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 11<<20)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		if r.MultipartForm != nil {
			r.MultipartForm.RemoveAll()
		}
		http.Error(w, "Choose an invoice file up to 10 MB", 400)
		return
	}
	defer r.MultipartForm.RemoveAll()
	var p core.PurchaseInvoice
	if err := json.Unmarshal([]byte(r.FormValue("invoice")), &p); err != nil {
		http.Error(w, "Invalid invoice details", 400)
		return
	}
	p.ID = 0
	p.FileName = ""
	if r.Method == "PUT" {
		id, err := strconv.Atoi(mux.Vars(r)["id"])
		if err != nil || id <= 0 {
			http.Error(w, "Invalid invoice ID", 400)
			return
		}
		p.ID = id
	}
	file, header, err := r.FormFile("file")
	if err != nil && !errors.Is(err, http.ErrMissingFile) {
		http.Error(w, "Could not read upload", 400)
		return
	}
	if err == nil {
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, (10<<20)+1))
		if err != nil || len(data) == 0 || len(data) > 10<<20 {
			http.Error(w, "Choose a non-empty invoice file up to 10 MB", 400)
			return
		}
		typ := http.DetectContentType(data)
		if typ != "application/pdf" && typ != "image/jpeg" && typ != "image/png" {
			http.Error(w, "Upload a PDF, JPEG or PNG invoice", 400)
			return
		}
		p.FileData = data
		p.FileType = typ
		p.FileName = filepath.Base(strings.ReplaceAll(header.Filename, `\`, "/"))
	}
	if err = core.SavePurchaseInvoice(s.DB, &p); err != nil {
		status := 400
		if errors.Is(err, sql.ErrNoRows) {
			status = 404
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "POST" {
		w.WriteHeader(201)
	}
	json.NewEncoder(w).Encode(p)
}
func (s *Server) DownloadPurchaseInvoice(w http.ResponseWriter, r *http.Request) {
	var name, typ string
	var data []byte
	err := s.DB.QueryRow(`SELECT file_name,file_type,file_data FROM purchase_invoices WHERE id=?`, mux.Vars(r)["id"]).Scan(&name, &typ, &data)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && len(data) == 0) {
		http.Error(w, "Invoice upload not found", 404)
		return
	}
	if err != nil {
		http.Error(w, "Could not load upload", 500)
		return
	}
	w.Header().Set("Content-Type", typ)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Write(data)
}

func (s *Server) LinkPurchaseInvoice(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil || id <= 0 {
		http.Error(w, "Invalid invoice ID", 400)
		return
	}
	var payload struct {
		TransactionID int `json:"bank_transaction_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&payload); err != nil || payload.TransactionID <= 0 {
		http.Error(w, "Choose a withdrawal", 400)
		return
	}
	if err := core.LinkPurchaseInvoice(s.DB, id, payload.TransactionID); err != nil {
		status := 400
		if errors.Is(err, sql.ErrNoRows) {
			status = 404
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Purchase invoice linked"})
}

func (s *Server) GetPurchaseParty(w http.ResponseWriter, r *http.Request) {
	gstin := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("gstin")))
	if len(gstin) != 15 {
		http.Error(w, "Enter a 15-character GSTIN", 400)
		return
	}
	p, err := core.GetPurchaseParty(s.DB, gstin)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "No saved party for this GSTIN", 404)
		return
	}
	if err != nil {
		http.Error(w, "Could not look up party", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(p)
}
