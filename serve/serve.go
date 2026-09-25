// Package serve runs Mamori providers as RPC servers for secret-resolver.
package serve

import (
	"context"
	"fmt"
	"io"
	"net/rpc"
	"runtime/debug"

	"github.com/xavidop/mamori"
	mrpc "github.com/yaronf/secret-resolver/rpc"
)

// Options configure Serve.
type Options struct {
	Name    string // Info.ProviderName
	Version string // Info.ProviderVersion; default: build info
	// Conn is the RPC duplex. If nil, the inherited fd from MAMORI_RPC_FD
	// (default 3 / ExtraFiles[0]) is used — stdout stays free for logging.
	Conn io.ReadWriteCloser
}

// Serve exposes the given providers over gob net/rpc on the inherited RPC fd.
func Serve(providers ...mamori.Provider) error {
	return ServeWith(Options{}, providers...)
}

// ServeRegistered serves whatever is in this process's mamori registry.
// In the out-of-process model that is normally a single blank-imported
// provider package (its init called Register) — not the full Mamori catalog.
// Requires mamori.Providers() (exported registry snapshot).
func ServeRegistered(opts ...Options) error {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	ps := mamori.Providers()
	if len(ps) == 0 {
		return fmt.Errorf("serve: no providers registered (blank-import a provider package?)")
	}
	return ServeWith(o, ps...)
}

// ServeWith is Serve with explicit naming / connection.
func ServeWith(opts Options, providers ...mamori.Provider) error {
	if len(providers) == 0 {
		return fmt.Errorf("serve: no providers")
	}
	byScheme := make(map[string]mamori.Provider, len(providers))
	schemes := make([]string, 0, len(providers))
	for _, p := range providers {
		if p == nil {
			return fmt.Errorf("serve: nil provider")
		}
		s := p.Scheme()
		if s == "" {
			return fmt.Errorf("serve: empty scheme")
		}
		if _, dup := byScheme[s]; dup {
			return fmt.Errorf("serve: duplicate scheme %q", s)
		}
		byScheme[s] = p
		schemes = append(schemes, s)
	}
	if opts.Name == "" {
		opts.Name = schemes[0]
	}
	if opts.Version == "" {
		opts.Version = buildVersion()
	}

	conn := opts.Conn
	if conn == nil {
		f, err := mrpc.OpenInherited()
		if err != nil {
			return err
		}
		defer f.Close()
		conn = f
	}

	svc := &service{
		name:     opts.Name,
		version:  opts.Version,
		byScheme: byScheme,
		schemes:  schemes,
	}
	srv := rpc.NewServer()
	if err := srv.RegisterName(mrpc.ServiceName, svc); err != nil {
		return err
	}
	mrpc.ServeConn(srv, conn)
	for _, p := range providers {
		if c, ok := p.(io.Closer); ok {
			_ = c.Close()
		}
	}
	return nil
}

func buildVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}

type service struct {
	name     string
	version  string
	byScheme map[string]mamori.Provider
	schemes  []string
}

func (s *service) Info(_ *mrpc.InfoRequest, reply *mrpc.InfoResponse) error {
	*reply = mrpc.InfoResponse{
		ProtocolVersion: mrpc.ProtocolVersion,
		ProviderName:    s.name,
		ProviderVersion: s.version,
		Schemes:         append([]string(nil), s.schemes...),
	}
	return nil
}

func (s *service) Resolve(args *mrpc.ResolveRequest, reply *mrpc.ResolveResult) error {
	*reply = mrpc.ResolveResult{}
	if args == nil || args.Ref == "" {
		reply.Err = &mrpc.RPCError{Kind: string(mamori.KindInvalid), Message: "empty ref"}
		return nil
	}
	ref, err := mamori.ParseRef(args.Ref)
	if err != nil {
		reply.Err = &mrpc.RPCError{Kind: string(mamori.KindInvalid), Message: err.Error()}
		return nil
	}
	p, ok := s.byScheme[ref.Scheme]
	if !ok {
		reply.Err = &mrpc.RPCError{Kind: string(mamori.KindInvalid), Message: fmt.Sprintf("scheme %q not served by this process", ref.Scheme)}
		return nil
	}

	ctx := context.Background()
	var cancel context.CancelFunc
	if args.Deadline != nil {
		ctx, cancel = context.WithDeadline(context.Background(), *args.Deadline)
		defer cancel()
	}

	val, err := p.Resolve(ctx, ref)
	if err != nil {
		reply.Err = mapErr(err)
		return nil
	}
	if len(val.Bytes) > mrpc.DefaultMaxValueBytes {
		// Not a mamori Kind; host NormalizeKind keeps "protocol".
		reply.Err = &mrpc.RPCError{
			Kind:    "protocol",
			Message: fmt.Sprintf("value exceeds max size (%d bytes)", mrpc.DefaultMaxValueBytes),
		}
		return nil
	}
	reply.OK = true
	reply.Value = mrpc.ResolveResponse{
		Bytes:     val.Bytes,
		Version:   val.Version,
		Sensitive: val.Sensitive,
		NotAfter:  val.NotAfter,
		Metadata:  val.Metadata,
	}
	return nil
}

func mapErr(err error) *mrpc.RPCError {
	kind := mamori.ErrorKind(err)
	if kind == "" {
		kind = mamori.KindUnknown
	}
	msg := err.Error()
	const max = 512
	if len(msg) > max {
		msg = msg[:max] + "…"
	}
	return &mrpc.RPCError{Kind: string(kind), Message: msg}
}
