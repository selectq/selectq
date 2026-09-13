package handlers

import (
	"database/sql"
	"errors"
	"fmt"
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
	inv, err := core.GetSalesInvoiceByID(s.DB, id)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Invoice not found", 404)
		return
	}
	if err != nil {
		http.Error(w, "Could not load invoice", 500)
		return
	}
	company, err := core.GetCompanyProfile(s.DB)
	if err != nil {
		http.Error(w, "Could not load company profile", 500)
		return
	}
	var contact core.BusinessPartnerContact
	if inv.ContactID != nil {
		err = s.DB.QueryRow(`SELECT name,email,phone FROM business_partner_contacts WHERE id=? AND business_partner_id=?`, *inv.ContactID, inv.BusinessPartnerID).Scan(&contact.Name, &contact.Email, &contact.Phone)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Could not load invoice contact", 500)
			return
		}
	}
	pdf, err := core.SalesInvoicePDF(inv, company, contact)
	if err != nil {
		http.Error(w, "Could not generate invoice PDF", 500)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "Invoice-" + strings.ReplaceAll(inv.InvoiceNumber, "/", "-") + ".pdf"}))
	w.Header().Set("Content-Length", fmt.Sprint(len(pdf)))
	w.Header().Set("Cache-Control", "no-store")
	w.Write(pdf)
}
