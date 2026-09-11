package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/gorilla/mux"
	"github.com/selectq/selectq/core"
	"io"
	"net/http"
	"path/filepath"
	"strings"
)

func (s *Server) GetReferenceRates(w http.ResponseWriter, r *http.Request) {
	rates, err := core.GetReferenceRates(s.DB)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rates)
}

func (s *Server) SaveReferenceRate(w http.ResponseWriter, r *http.Request) {
	var rate core.ReferenceRate
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	if err := decoder.Decode(&rate); err != nil {
		http.Error(w, "Invalid reference rate JSON", 400)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "Expected a single JSON object", 400)
		return
	}
	var original *core.ReferenceRate
	status := http.StatusCreated
	if r.Method == http.MethodPut {
		vars := mux.Vars(r)
		original = &core.ReferenceRate{RateDate: vars["date"], Currency: vars["currency"]}
		status = http.StatusOK
	}
	saved, err := core.SaveReferenceRate(s.DB, rate, original)
	if err != nil {
		code := http.StatusInternalServerError
		switch {
		case errors.Is(err, sql.ErrNoRows):
			code = http.StatusNotFound
		case errors.Is(err, core.ErrReferenceRateExists):
			code = http.StatusConflict
		case errors.Is(err, core.ErrInvalidReferenceRate):
			code = http.StatusBadRequest
		}
		http.Error(w, err.Error(), code)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(saved)
}

func (s *Server) UploadReferenceRates(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	err := r.ParseMultipartForm(2 << 20)
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if err != nil {
		code := http.StatusBadRequest
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			code = http.StatusRequestEntityTooLarge
		}
		http.Error(w, "Upload a valid XLSX workbook (maximum request size 10 MB)", code)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Choose a reference rate workbook", 400)
		return
	}
	defer file.Close()
	if !strings.EqualFold(filepath.Ext(header.Filename), ".xlsx") {
		http.Error(w, "Reference rates require an .xlsx workbook", 400)
		return
	}
	sheet := strings.TrimSpace(r.FormValue("sheet"))
	result, err := core.ImportReferenceRatesReader(s.DB, file, header.Filename, sheet)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
