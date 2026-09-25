package secretresolver_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	secretresolver "github.com/yaronf/secret-resolver"
)

func TestResolveViaFakeProvider(t *testing.T) {
	fake := buildFake(t)
	values, _ := json.Marshal(map[string]string{
		"fake://hello": "world",
		"fake://bin":   "a\x00b",
	})

	r, err := secretresolver.New(secretresolver.WithProviders(secretresolver.Provider{
		Command: fake,
		Env:     map[string]string{"FAKE_VALUES": string(values)},
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	v, err := r.Resolve(ctx, "fake://hello")
	if err != nil {
		t.Fatal(err)
	}
	if string(v.Bytes) != "world" {
		t.Fatalf("got %q", v.Bytes)
	}

	v, err = r.Resolve(ctx, "fake://bin")
	if err != nil {
		t.Fatal(err)
	}
	if string(v.Bytes) != "a\x00b" {
		t.Fatalf("binary value = %q", v.Bytes)
	}

	_, err = r.Resolve(ctx, "fake://missing")
	if !errors.Is(err, secretresolver.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestConcurrentResolve(t *testing.T) {
	fake := buildFake(t)
	m := map[string]string{}
	for i := 0; i < 20; i++ {
		m["fake://k"+strconv.Itoa(i)] = "v" + strconv.Itoa(i)
	}
	b, _ := json.Marshal(m)

	r, err := secretresolver.New(secretresolver.WithProviders(secretresolver.Provider{
		Command: fake,
		Env:     map[string]string{"FAKE_VALUES": string(b)},
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	var wg sync.WaitGroup
	errCh := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			v, err := r.Resolve(ctx, "fake://k"+strconv.Itoa(i))
			if err != nil {
				errCh <- err
				return
			}
			if string(v.Bytes) != "v"+strconv.Itoa(i) {
				errCh <- errors.New("bad value for " + strconv.Itoa(i))
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
}

func TestUnknownScheme(t *testing.T) {
	fake := buildFake(t)
	r, err := secretresolver.New(secretresolver.WithProviders(secretresolver.Provider{
		Command: fake,
		Env:     map[string]string{"FAKE_VALUES": `{"fake://x":"y"}`},
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	_, err = r.Resolve(context.Background(), "other://x")
	var re *secretresolver.Error
	if !errors.As(err, &re) || re.Kind != secretresolver.KindInvalid {
		t.Fatalf("got %v", err)
	}
}

func buildFake(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "mamori-provider-fake")
	cmd := exec.Command("go", "build", "-o", out, "./cmd/mamori-provider-fake")
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if outb, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake: %v\n%s", err, outb)
	}
	return out
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return wd
}
