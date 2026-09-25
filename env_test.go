package secretresolver

import (
	"strings"
	"testing"

	"github.com/yaronf/secret-resolver/rpc"
)

func TestBuildChildEnvAllowlistAndRPCFDLast(t *testing.T) {
	t.Setenv("PATH", "/bin")
	t.Setenv("HOME", "/home/test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "should-not-leak")
	t.Setenv("VAULT_TOKEN", "should-not-leak")
	t.Setenv(rpc.EnvRPCFD, "999") // must not win over our append

	env, err := buildChildEnv(map[string]string{
		"SQLITE_PATH": "/tmp/db",
		"PATH":        "/custom/bin", // overrides allowlisted PATH
	})
	if err != nil {
		t.Fatal(err)
	}

	joined := strings.Join(env, "\n")
	if strings.Contains(joined, "AWS_SECRET_ACCESS_KEY") || strings.Contains(joined, "VAULT_TOKEN") {
		t.Fatalf("parent secrets leaked into child env:\n%s", joined)
	}
	if !strings.Contains(joined, "SQLITE_PATH=/tmp/db") {
		t.Fatalf("missing explicit env:\n%s", joined)
	}
	if !strings.Contains(joined, "PATH=/custom/bin") {
		t.Fatalf("Provider.Env should override allowlist PATH:\n%s", joined)
	}
	if strings.Contains(joined, "PATH=/bin\n") || strings.HasSuffix(joined, "PATH=/bin") {
		// ensure we didn't also keep the old PATH as a duplicate entry
		count := 0
		for _, kv := range env {
			if strings.HasPrefix(kv, "PATH=") {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("duplicate PATH entries: %d\n%s", count, joined)
		}
	}
	last := env[len(env)-1]
	want := rpc.EnvRPCFD + "=" + "3"
	if last != want {
		t.Fatalf("RPC fd must be last: got %q want %q", last, want)
	}
	for i, kv := range env[:len(env)-1] {
		if strings.HasPrefix(kv, rpc.EnvRPCFD+"=") {
			t.Fatalf("stale %s at [%d]=%q", rpc.EnvRPCFD, i, kv)
		}
	}
}

func TestBuildChildEnvRejectsRPCFDOverride(t *testing.T) {
	_, err := buildChildEnv(map[string]string{rpc.EnvRPCFD: "7"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNormalizeKind(t *testing.T) {
	if NormalizeKind("not_found") != KindNotFound {
		t.Fatal()
	}
	if NormalizeKind("totally_bogus") != KindUnknown {
		t.Fatal()
	}
}

func TestTruncateMessage(t *testing.T) {
	if truncateMessage("ok") != "ok" {
		t.Fatal()
	}
	long := strings.Repeat("a", 600)
	s := truncateMessage(long)
	if len(s) != 512+len("…") {
		t.Fatalf("got len %d", len(s))
	}
}
