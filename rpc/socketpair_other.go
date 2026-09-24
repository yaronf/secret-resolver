//go:build !unix

package rpc

import (
	"fmt"
	"os"
)

// NewSocketPair is only implemented on Unix (ExtraFiles + socketpair).
func NewSocketPair() (parent, child *os.File, err error) {
	return nil, nil, fmt.Errorf("rpc: dedicated RPC fd requires Unix socketpair")
}
