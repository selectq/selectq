package handlers

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/selectq/selectq/core"
)

func (s *Server) DownloadSalesInvoicePDF(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil || id <= 0 {
		http.Error(w, "Invalid invoice ID", 400)
		return
	}
	pdf, filename, status, err := s.salesInvoicePDF(id)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	w.Header().Set("Content-Length", fmt.Sprint(len(pdf)))
	w.Header().Set("Cache-Control", "no-store")
	w.Write(pdf)
}

func (s *Server) salesInvoicePDF(id int) ([]byte, string, int, error) {
	inv, err := core.GetSalesInvoiceByID(s.DB, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", 404, errors.New("Invoice not found")
	}
	if err != nil {
		return nil, "", 500, errors.New("Could not load invoice")
	}
	company, err := core.GetCompanyProfile(s.DB)
	if err != nil {
		return nil, "", 500, errors.New("Could not load company profile")
	}
	var contact core.BusinessPartnerContact
	if inv.ContactID != nil {
		err = s.DB.QueryRow(`SELECT name,email,phone FROM business_partner_contacts WHERE id=? AND business_partner_id=?`, *inv.ContactID, inv.BusinessPartnerID).Scan(&contact.Name, &contact.Email, &contact.Phone)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, "", 500, errors.New("Could not load invoice contact")
		}
	}
	pdf, err := core.SalesInvoicePDF(inv, company, contact)
	if err != nil {
		return nil, "", 500, errors.New("Could not generate invoice PDF")
	}
	filename := "Invoice-" + strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, inv.InvoiceNumber) + ".pdf"
	return pdf, filename, 200, nil
}

func (s *Server) DownloadSalesInvoicesZIP(w http.ResponseWriter, r *http.Request) {
	var request struct {
		IDs []int `json:"ids"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
	if err := decoder.Decode(&request); err != nil || len(request.IDs) == 0 || len(request.IDs) > 100 {
		http.Error(w, "Select between 1 and 100 invoices", 400)
		return
	}
	if decoder.Decode(new(any)) != io.EOF {
		http.Error(w, "Invalid request", 400)
		return
	}
	for _, id := range request.IDs {
		if id <= 0 {
			http.Error(w, "Invalid invoice ID", 400)
			return
		}
	}
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	seen := make(map[int]bool)
	names := make(map[string]bool)
	for _, id := range request.IDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		pdf, filename, status, err := s.salesInvoicePDF(id)
		if err != nil {
			http.Error(w, err.Error(), status)
			return
		}
		baseName := strings.TrimSuffix(filename, ".pdf")
		for suffix := 1; names[filename]; suffix++ {
			filename = baseName + fmt.Sprintf("-%d-%d.pdf", id, suffix)
		}
		names[filename] = true
		entry, err := archive.Create(filename)
		if err == nil {
			_, err = entry.Write(pdf)
		}
		if err != nil {
			http.Error(w, "Could not create invoice ZIP", 500)
			return
		}
	}
	if err := archive.Close(); err != nil {
		http.Error(w, "Could not create invoice ZIP", 500)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="Invoices.zip"`)
	w.Header().Set("Content-Length", fmt.Sprint(output.Len()))
	w.Header().Set("Cache-Control", "no-store")
	w.Write(output.Bytes())
}
