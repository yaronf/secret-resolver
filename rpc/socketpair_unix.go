//go:build unix

package rpc

import (
	"fmt"
	"os"
	"syscall"
)

// NewSocketPair returns a connected Unix stream pair for parent↔child RPC.
// Pass child to exec.Cmd.ExtraFiles[0]; keep parent for the rpc.Client.
func NewSocketPair() (parent, child *os.File, err error) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("rpc: socketpair: %w", err)
	}
	// Socketpair fds are CLOEXEC; ExtraFiles still inherits them into the child.
	parent = os.NewFile(uintptr(fds[0]), "mamori-rpc-parent")
	child = os.NewFile(uintptr(fds[1]), "mamori-rpc-child")
	if parent == nil || child == nil {
		if parent != nil {
			_ = parent.Close()
		}
		if child != nil {
			_ = child.Close()
		}
		return nil, nil, fmt.Errorf("rpc: NewFile for socketpair failed")
	}
	return parent, child, nil
}
