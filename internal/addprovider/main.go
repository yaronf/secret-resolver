// Command addprovider appends a provider to providers.manifest.json and regenerates mains.
//
//	go run ./internal/addprovider -import github.com/xavidop/mamori/providers/vault
//	go run ./internal/addprovider -import github.com/xavidop/mamori/providers/aws \
//	  -new NewSecretsManager() -new NewParameterStore() -new NewAppConfig()
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type manifest struct {
	Providers []provider `json:"providers"`
}

type provider struct {
	Name   string   `json:"name"`
	Import string   `json:"import"`
	New    []string `json:"new"`
}

func main() {
	var (
		name   = flag.String("name", "", "binary/provider name (default: last path segment of -import)")
		imp    = flag.String("import", "", "Go import path, e.g. github.com/xavidop/mamori/providers/vault")
		news   stringList
		noGen  = flag.Bool("nogen", false, "update manifest only; skip go generate")
	)
	flag.Var(&news, "new", "constructor call to serve, e.g. New() (repeatable; default New())")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: go run ./internal/addprovider -import <path> [-name <name>] [-new New()]...

Adds a Mamori provider package to providers.manifest.json and runs go generate
unless -nogen is set.

Examples:
  go run ./internal/addprovider -import github.com/xavidop/mamori/providers/vault
  go run ./internal/addprovider -import github.com/xavidop/mamori/providers/aws \
    -new NewSecretsManager() -new NewParameterStore() -new NewAppConfig()

`)
		flag.PrintDefaults()
	}
	flag.Parse()
	if *imp == "" {
		flag.Usage()
		os.Exit(2)
	}

	root, err := findRoot()
	if err != nil {
		fatal(err)
	}

	pName := *name
	if pName == "" {
		pName = filepath.Base(*imp)
	}
	ctors := []string(news)
	if len(ctors) == 0 {
		ctors = []string{"New()"}
	}
	for i, c := range ctors {
		ctors[i] = strings.TrimSpace(c)
		if ctors[i] == "" {
			fatal(fmt.Errorf("empty -new"))
		}
	}

	path := filepath.Join(root, "providers.manifest.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		fatal(err)
	}
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		fatal(err)
	}
	for _, existing := range m.Providers {
		if existing.Name == pName {
			fatal(fmt.Errorf("provider %q already in manifest", pName))
		}
	}
	m.Providers = append(m.Providers, provider{
		Name:   pName,
		Import: *imp,
		New:    ctors,
	})

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(m); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		fatal(err)
	}
	fmt.Println("updated", path)

	if !*noGen {
		cmd := exec.Command("go", "generate", ".")
		cmd.Dir = root
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fatal(fmt.Errorf("go generate: %w", err))
		}
		fmt.Printf("next: cd cmd/mamori-provider-%s && go mod tidy && go build\n", pName)
	}
}

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ", ") }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func findRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "providers.manifest.json")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("providers.manifest.json not found")
		}
		dir = parent
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "addprovider:", err)
	os.Exit(1)
}
