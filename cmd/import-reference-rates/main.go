package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/selectq/selectq/core"
)

func main() {
	dbPath := flag.String("db", "bank_statements.db", "SQLite database path")
	file := flag.String("file", "", "Reference rate XLSX workbook (required)")
	sheet := flag.String("sheet", "", "Worksheet override (automatically detected when omitted)")
	flag.Parse()
	if *file == "" {
		log.Fatal("provide -file ReferenceRate.xlsx")
	}
	db, err := core.InitDB(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	result, err := core.ImportReferenceRates(db, *file, *sheet)
	if err != nil {
		log.Fatal(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		log.Fatal(err)
	}
}
