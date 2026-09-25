package rpc

import (
	"fmt"
	"os"
	"strconv"
)

// EnvRPCFD is the child environment variable naming the inherited RPC file
// descriptor (first ExtraFiles entry → fd 3 by default).
const EnvRPCFD = "SECRET_RESOLVER_RPC_FD"

// DefaultRPCFD is the fd number for cmd.ExtraFiles[0] (stdin=0, stdout=1, stderr=2).
const DefaultRPCFD = 3

// OpenInherited opens the RPC duplex inherited from the parent via ExtraFiles.
// Stdout/stderr remain free for normal logging.
func OpenInherited() (*os.File, error) {
	fd := DefaultRPCFD
	if s := os.Getenv(EnvRPCFD); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("rpc: bad %s=%q", EnvRPCFD, s)
		}
		fd = n
	}
	f := os.NewFile(uintptr(fd), "secret-resolver-rpc")
	if f == nil {
		return nil, fmt.Errorf("rpc: NewFile(%d) failed", fd)
	}
	// Touch the fd so a missing ExtraFiles fails fast with a clear error.
	if _, err := f.Stat(); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("rpc: inherited fd %d: %w (parent must pass ExtraFiles)", fd, err)
	}
	return f, nil
}
