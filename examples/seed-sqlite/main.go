package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

func main() {
	dbPath := flag.String("db", "/tmp/mamori-demo.db", "sqlite path")
	flag.Parse()
	db, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS config (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		fatal(err)
	}
	if _, err := db.Exec(`INSERT OR REPLACE INTO config(key, value) VALUES (?, ?)`, "greeting", "hello-from-sqlite"); err != nil {
		fatal(err)
	}
	fmt.Println("seeded", *dbPath)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
