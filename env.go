package secretresolver

import (
	"fmt"
	"os"
	"strconv"

	"github.com/yaronf/secret-resolver/rpc"
)

// parentEnvAllowlist is copied into provider children. Everything else from the
// host process (cloud tokens, shell secrets, etc.) stays out unless the caller
// puts it in Provider.Env explicitly.
var parentEnvAllowlist = map[string]struct{}{
	"PATH":            {},
	"HOME":            {},
	"USER":            {},
	"LOGNAME":         {},
	"LANG":            {},
	"LC_ALL":          {},
	"LC_CTYPE":        {},
	"TZ":              {},
	"TMPDIR":          {},
	"TMP":             {},
	"TEMP":            {},
	"XDG_RUNTIME_DIR": {},
	"SSL_CERT_FILE":   {},
	"SSL_CERT_DIR":    {},
	"REQUESTS_CA_BUNDLE": {},
	"CURL_CA_BUNDLE":  {},
}

// buildChildEnv returns the environment for a provider process.
// Order: allowlisted parent vars, then Provider.Env, then MAMORI_RPC_FD last
// (always wins). Provider.Env must not set MAMORI_RPC_FD.
func buildChildEnv(providerEnv map[string]string) ([]string, error) {
	if _, ok := providerEnv[rpc.EnvRPCFD]; ok {
		return nil, fmt.Errorf("%w: Provider.Env must not set %s", ErrInvalid, rpc.EnvRPCFD)
	}

	out := make([]string, 0, len(parentEnvAllowlist)+len(providerEnv)+1)
	for _, kv := range os.Environ() {
		k, _, ok := splitEnv(kv)
		if !ok {
			continue
		}
		if _, allow := parentEnvAllowlist[k]; allow {
			out = append(out, kv)
		}
	}
	for k, v := range providerEnv {
		if k == "" {
			return nil, fmt.Errorf("%w: empty env key", ErrInvalid)
		}
		out = append(out, k+"="+v)
	}
	// Always last so nothing can shadow the inherited ExtraFiles fd number.
	out = append(out, rpc.EnvRPCFD+"="+strconv.Itoa(rpc.DefaultRPCFD))
	return out, nil
}

func splitEnv(kv string) (key, val string, ok bool) {
	for i := 0; i < len(kv); i++ {
		if kv[i] == '=' {
			return kv[:i], kv[i+1:], true
		}
	}
	return "", "", false
}
