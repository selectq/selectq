package handlers

import (
	"context"
	"encoding/json"
	"github.com/selectq/selectq/core"
	"io"
	"net/http"
	"time"
)

func (s *Server) ParsePurchaseInvoice(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 11<<20)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		if r.MultipartForm != nil {
			r.MultipartForm.RemoveAll()
		}
		http.Error(w, "Upload a PDF up to 10 MB", 400)
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Choose an invoice PDF", 400)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (10<<20)+1))
	if err != nil || len(data) == 0 || len(data) > 10<<20 || http.DetectContentType(data) != "application/pdf" {
		http.Error(w, "Upload a PDF up to 10 MB", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	p, err := core.ParseBankPurchasePDF(ctx, data)
	if err != nil {
		http.Error(w, err.Error(), 422)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}
