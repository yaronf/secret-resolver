package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	resolver "github.com/yaronf/mamori-resolver"
)

func main() {
	var (
		providers stringList
		envs      stringList
		timeout   = flag.Duration("timeout", 30*time.Second, "per-resolve timeout")
	)
	flag.Var(&providers, "provider", "provider command (repeatable)")
	flag.Var(&envs, "env", "KEY=VALUE for provider processes (repeatable)")
	flag.Parse()
	if len(providers) == 0 || flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "usage: mamori-resolver -provider ./mamori-provider-sqlite [-env KEY=VAL] <uri>\n")
		os.Exit(2)
	}

	envMap := map[string]string{}
	for _, e := range envs {
		k, v, ok := strings.Cut(e, "=")
		if !ok || k == "" {
			fmt.Fprintf(os.Stderr, "resolver: bad -env %q (want KEY=VALUE)\n", e)
			os.Exit(2)
		}
		envMap[k] = v
	}

	ps := make([]resolver.Provider, 0, len(providers))
	for _, cmd := range providers {
		ps = append(ps, resolver.Provider{Command: cmd, Env: envMap})
	}

	r, err := resolver.New(resolver.WithProviders(ps...))
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

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}
