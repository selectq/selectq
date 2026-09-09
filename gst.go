package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/selectq/selectq/core"
	"github.com/selectq/selectq/handlers"
	"github.com/gorilla/mux"
)

func main() {
	dbPath := "bank_statements.db"

	// Initialize Database
	db, err := core.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize Gorilla Mux Router
	r := mux.NewRouter()

	// Initialize the Handlers Server
	srv := handlers.NewServer(db)

	// --- API Endpoints ---
	api := r.PathPrefix("/api").Subrouter()
	
	api.HandleFunc("/imports", srv.GetImports).Methods("GET")
	api.HandleFunc("/imports/{id:[0-9]+}/transactions", srv.GetTransactions).Methods("GET")
	api.HandleFunc("/transactions/{id:[0-9]+}", srv.UpdateTransaction).Methods("PUT")
	
	api.HandleFunc("/business-partners", srv.GetBusinessPartners).Methods("GET")
	api.HandleFunc("/business-partners", srv.CreateBusinessPartner).Methods("POST")
	api.HandleFunc("/business-partners/{id:[0-9]+}", srv.GetBusinessPartnerByID).Methods("GET")
	api.HandleFunc("/business-partners/{id:[0-9]+}", srv.UpdateBusinessPartner).Methods("PUT")
	
	api.HandleFunc("/sales-invoices", srv.GetSalesInvoices).Methods("GET")
	api.HandleFunc("/sales-invoices", srv.CreateSalesInvoice).Methods("POST")
	api.HandleFunc("/sales-invoices/{id:[0-9]+}", srv.UpdateSalesInvoice).Methods("PUT")
	
	api.HandleFunc("/upload", srv.UploadStatement).Methods("POST")

	// --- Serve Angular Frontend ---
	// Angular 17 output path is usually dist/frontend/browser
	fs := http.FileServer(http.Dir("frontend/dist/frontend/browser"))
	
	// Create a catch-all route for static files and Angular SPA routing
	r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// If file does not exist, serve index.html (for Angular routing)
		path := filepath.Join("frontend", "dist", "frontend", "browser", filepath.Clean(req.URL.Path))
		if stat, err := os.Stat(path); os.IsNotExist(err) || stat.IsDir() {
			http.ServeFile(w, req, "frontend/dist/frontend/browser/index.html")
			return
		}
		fs.ServeHTTP(w, req)
	})

	port := ":8080"
	fmt.Printf("Server starting on http://localhost%s\n", port)
	if err := http.ListenAndServe(port, r); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
