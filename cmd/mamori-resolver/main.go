package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	resolver "github.com/yaronf/mamori-resolver"
)

func main() {
	cfg := flag.String("config", "", "path to JSON config listing provider commands")
	timeout := flag.Duration("timeout", 30*time.Second, "per-resolve timeout")
	flag.Parse()
	if *cfg == "" || flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "usage: mamori-resolver -config providers.json <uri>\n")
		os.Exit(2)
	}

	r, err := resolver.New(resolver.WithConfigFile(*cfg))
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolver: %v\n", err)
		os.Exit(1)
	}
	defer r.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	v, err := r.Resolve(ctx, flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve: %v\n", err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(struct {
		Bytes     string            `json:"bytes"`
		Version   string            `json:"version"`
		Sensitive bool              `json:"sensitive"`
		Metadata  map[string]string `json:"metadata,omitempty"`
	}{
		Bytes:     string(v.Bytes),
		Version:   v.Version,
		Sensitive: v.Sensitive,
		Metadata:  v.Metadata,
	})
}
