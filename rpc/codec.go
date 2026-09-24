package rpc

import (
	"encoding/gob"
	"io"
	"net/rpc"
)

// stdioConn is a full-duplex ReadWriteCloser over separate reader/writer
// (child stdin/stdout or parent pipes). Close closes both ends.
type stdioConn struct {
	r io.ReadCloser
	w io.WriteCloser
}

func NewStdioConn(r io.ReadCloser, w io.WriteCloser) io.ReadWriteCloser {
	return &stdioConn{r: r, w: w}
}

func (c *stdioConn) Read(p []byte) (int, error)  { return c.r.Read(p) }
func (c *stdioConn) Write(p []byte) (int, error) { return c.w.Write(p) }
func (c *stdioConn) Close() error {
	errW := c.w.Close()
	errR := c.r.Close()
	if errW != nil {
		return errW
	}
	return errR
}

// gob codecs match stdlib net/rpc's gobClientCodec / gobServerCodec: no mutex.
// Concurrent safety comes from rpc.Client.reqMutex (WriteRequest) and
// Server.sendResponse's sending mutex (WriteResponse). Reads are single-goroutine.
type gobServerCodec struct {
	rwc io.ReadWriteCloser
	dec *gob.Decoder
	enc *gob.Encoder
}

func newGobServerCodec(conn io.ReadWriteCloser) *gobServerCodec {
	return &gobServerCodec{
		rwc: conn,
		dec: gob.NewDecoder(conn),
		enc: gob.NewEncoder(conn),
	}
}

func (c *gobServerCodec) ReadRequestHeader(r *rpc.Request) error {
	return c.dec.Decode(r)
}

func (c *gobServerCodec) ReadRequestBody(body any) error {
	return c.dec.Decode(body)
}

func (c *gobServerCodec) WriteResponse(r *rpc.Response, body any) error {
	if err := c.enc.Encode(r); err != nil {
		return err
	}
	return c.enc.Encode(body)
}

func (c *gobServerCodec) Close() error { return c.rwc.Close() }

type gobClientCodec struct {
	rwc io.ReadWriteCloser
	dec *gob.Decoder
	enc *gob.Encoder
}

func newGobClientCodec(conn io.ReadWriteCloser) *gobClientCodec {
	return &gobClientCodec{
		rwc: conn,
		dec: gob.NewDecoder(conn),
		enc: gob.NewEncoder(conn),
	}
}

func (c *gobClientCodec) WriteRequest(r *rpc.Request, body any) error {
	if err := c.enc.Encode(r); err != nil {
		return err
	}
	return c.enc.Encode(body)
}

func (c *gobClientCodec) ReadResponseHeader(r *rpc.Response) error {
	return c.dec.Decode(r)
}

func (c *gobClientCodec) ReadResponseBody(body any) error {
	return c.dec.Decode(body)
}

func (c *gobClientCodec) Close() error { return c.rwc.Close() }

// NewClient returns an rpc.Client over conn using gob.
func NewClient(conn io.ReadWriteCloser) *rpc.Client {
	return rpc.NewClientWithCodec(newGobClientCodec(conn))
}

// ServeConn serves the registered RPC receiver on conn until it closes.
func ServeConn(server *rpc.Server, conn io.ReadWriteCloser) {
	server.ServeCodec(newGobServerCodec(conn))
}
