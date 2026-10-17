package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/selectq/selectq/core"
)

func (s *Server) DownloadStatement(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil || id <= 0 {
		http.Error(w, "Invalid statement ID", 400)
		return
	}
	data, filename, err := core.StatementDownload(s.DB, id, ".")
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Statement not found", 404)
		return
	}
	if errors.Is(err, core.ErrStatementOriginalUnavailable) || errors.Is(err, core.ErrStatementMismatch) {
		http.Error(w, err.Error(), 409)
		return
	}
	if err != nil {
		http.Error(w, "Could not generate statement download", 500)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	w.Header().Set("Content-Length", fmt.Sprint(len(data)))
	w.Header().Set("Cache-Control", "no-store")
	w.Write(data)
}
