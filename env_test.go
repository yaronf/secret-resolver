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
	if !strings.Contains(joined, "PATH=/bin") || !strings.Contains(joined, "HOME=/home/test") {
		t.Fatalf("allowlist missing:\n%s", joined)
	}
	last := env[len(env)-1]
	want := rpc.EnvRPCFD + "=" + "3"
	if last != want {
		t.Fatalf("RPC fd must be last: got %q want %q", last, want)
	}
	// Parent's MAMORI_RPC_FD=999 must not appear, or if somehow present must be before last.
	for i, kv := range env[:len(env)-1] {
		if strings.HasPrefix(kv, rpc.EnvRPCFD+"=") && kv != want {
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

func TestSanitizeErrorMessage(t *testing.T) {
	s := sanitizeErrorMessage("ok\x00secret")
	if strings.Contains(s, "\x00") {
		t.Fatalf("control not stripped: %q", s)
	}
	long := strings.Repeat("a", 600)
	s = sanitizeErrorMessage(long)
	if len(s) > 520 {
		t.Fatalf("not truncated: %d", len(s))
	}
}
