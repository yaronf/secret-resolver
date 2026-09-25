package rpc

import (
	"errors"
	"fmt"
	"io"
)

// DefaultMaxConnRead is the max bytes the host will read from one provider
// connection (gob decode DoS bound). Cumulative for the process lifetime of
// that child.
const DefaultMaxConnRead int64 = 16 << 20 // 16 MiB

// DefaultMaxValueBytes is the max secret payload accepted from Resolve.
const DefaultMaxValueBytes = 1 << 20 // 1 MiB

// ErrTooLarge is returned when a size limit is exceeded.
var ErrTooLarge = errors.New("rpc: payload too large")

// LimitedConn wraps a ReadWriteCloser and caps total bytes read.
type LimitedConn struct {
	io.ReadWriteCloser
	remaining int64
}

// LimitReads returns a connection that errors after maxRead bytes have been read.
func LimitReads(conn io.ReadWriteCloser, maxRead int64) *LimitedConn {
	if maxRead <= 0 {
		maxRead = DefaultMaxConnRead
	}
	return &LimitedConn{ReadWriteCloser: conn, remaining: maxRead}
}

func (c *LimitedConn) Read(p []byte) (int, error) {
	if c.remaining <= 0 {
		return 0, fmt.Errorf("%w: connection read budget exhausted", ErrTooLarge)
	}
	if int64(len(p)) > c.remaining {
		p = p[:c.remaining]
	}
	n, err := c.ReadWriteCloser.Read(p)
	c.remaining -= int64(n)
	return n, err
}
