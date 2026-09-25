package secretresolver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainModuleHasNoProviderSDKs(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, bad := range []string{
		"aws-sdk",
		"modernc.org/sqlite",
		"hashicorp/vault",
		"github.com/xavidop/mamori",
	} {
		if strings.Contains(s, bad) {
			t.Fatalf("resolver go.mod must not require %q:\n%s", bad, s)
		}
	}
}
