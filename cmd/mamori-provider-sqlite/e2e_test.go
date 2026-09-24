package main_test

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	resolver "github.com/yaronf/mamori-resolver"
	"github.com/yaronf/mamori-resolver/config"
)

func TestResolveViaSQLiteProvider(t *testing.T) {
	repo := filepath.Join("..", "..")
	dbPath := filepath.Join(t.TempDir(), "demo.db")
	mustInitSQLite(t, dbPath)

	bin := filepath.Join(t.TempDir(), "mamori-provider-sqlite")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = "."
	if wd, err := os.Getwd(); err == nil {
		cmd.Dir = wd
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build sqlite provider: %v\n%s", err, out)
	}

	r, err := resolver.New(resolver.WithProviders(config.Provider{
		Command: bin,
		Env:     map[string]string{"SQLITE_PATH": dbPath},
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	v, err := r.Resolve(ctx, "sqlite://config/greeting")
	if err != nil {
		t.Fatal(err)
	}
	if string(v.Bytes) != "hello-from-sqlite" {
		t.Fatalf("got %q want hello-from-sqlite (repo=%s)", v.Bytes, repo)
	}
}

func mustInitSQLite(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE config (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO config(key, value) VALUES (?, ?)`, "greeting", "hello-from-sqlite"); err != nil {
		t.Fatal(err)
	}
}
