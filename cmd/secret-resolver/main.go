package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	secretresolver "github.com/yaronf/secret-resolver"
)

func main() {
	var (
		providers   stringList
		envs        stringList
		timeout     = flag.Duration("timeout", 30*time.Second, "per-resolve timeout")
		showSecrets = flag.Bool("show-secrets", false, "print secret bytes (default: redact)")
	)
	flag.Var(&providers, "provider", "provider command (repeatable)")
	flag.Var(&envs, "env", "KEY=VALUE for provider processes (repeatable)")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s -provider CMD [flags] <ref>\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if len(providers) == 0 || flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	}

	envMap := map[string]string{}
	for _, e := range envs {
		k, v, ok := strings.Cut(e, "=")
		if !ok || k == "" {
			fmt.Fprintf(os.Stderr, "secret-resolver: bad -env %q (want KEY=VALUE)\n", e)
			os.Exit(2)
		}
		envMap[k] = v
	}

	ps := make([]secretresolver.Provider, 0, len(providers))
	for _, cmd := range providers {
		ps = append(ps, secretresolver.Provider{Command: cmd, Env: envMap})
	}

	r, err := secretresolver.New(secretresolver.WithProviders(ps...))
	if err != nil {
		fmt.Fprintf(os.Stderr, "secret-resolver: %v\n", err)
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
	_ = enc.Encode(formatResolveJSON(v, *showSecrets))
}

type resolveJSON struct {
	Bytes     string            `json:"bytes,omitempty"`
	ByteLen   int               `json:"byte_len,omitempty"`
	Redacted  bool              `json:"redacted,omitempty"`
	Version   string            `json:"version"`
	Sensitive bool              `json:"sensitive"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

func formatResolveJSON(v secretresolver.Value, showSecrets bool) resolveJSON {
	out := resolveJSON{
		Version:   v.Version,
		Sensitive: v.Sensitive,
		Metadata:  v.Metadata,
	}
	if showSecrets {
		out.Bytes = string(v.Bytes)
		return out
	}
	out.Redacted = true
	out.ByteLen = len(v.Bytes)
	out.Bytes = redactStars(len(v.Bytes))
	return out
}

// redactStars is a visual stand-in for secret bytes (length capped so huge
// values don't flood the terminal). True size is in byte_len.
func redactStars(n int) string {
	const maxStars = 12
	if n <= 0 {
		return ""
	}
	if n > maxStars {
		n = maxStars
	}
	return strings.Repeat("*", n)
}

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}
