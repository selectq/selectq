package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/selectq/selectq/core"
)

func (s *Server) browserRequest(w http.ResponseWriter, r *http.Request, action func(context.Context, *sql.Conn) (any, error)) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	conn, closeConn, err := core.BrowserConnection(ctx, s.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer closeConn()
	result, err := action(ctx, conn)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		if errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusRequestTimeout
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
func (s *Server) BrowserObjects(w http.ResponseWriter, r *http.Request) {
	s.browserRequest(w, r, func(ctx context.Context, c *sql.Conn) (any, error) { return core.BrowserObjects(ctx, c) })
}
func (s *Server) BrowserDetail(w http.ResponseWriter, r *http.Request) {
	s.browserRequest(w, r, func(ctx context.Context, c *sql.Conn) (any, error) {
		return core.BrowserDetail(ctx, c, r.URL.Query().Get("name"))
	})
}
func (s *Server) BrowserSettings(w http.ResponseWriter, r *http.Request) {
	s.browserRequest(w, r, func(ctx context.Context, c *sql.Conn) (any, error) { return core.BrowserSettings(ctx, c) })
}
func (s *Server) BrowserRows(w http.ResponseWriter, r *http.Request) {
	limit, offset := 100, 0
	var err error
	if value := r.URL.Query().Get("limit"); value != "" {
		limit, err = strconv.Atoi(value)
		if err != nil {
			http.Error(w, "invalid limit", 400)
			return
		}
	}
	if value := r.URL.Query().Get("offset"); value != "" {
		offset, err = strconv.Atoi(value)
		if err != nil {
			http.Error(w, "invalid offset", 400)
			return
		}
	}
	s.browserRequest(w, r, func(ctx context.Context, c *sql.Conn) (any, error) {
		return core.BrowserTableRows(ctx, c, r.URL.Query().Get("name"), limit, offset)
	})
}
func (s *Server) BrowserSQL(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		SQL  string `json:"sql"`
		Mode string `json:"mode"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024))
	if err := decoder.Decode(&payload); err != nil {
		http.Error(w, "invalid SQL request: "+err.Error(), 400)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "expected one JSON request", 400)
		return
	}
	s.browserRequest(w, r, func(ctx context.Context, c *sql.Conn) (any, error) {
		return core.BrowserSQL(ctx, c, payload.SQL, payload.Mode)
	})
}
