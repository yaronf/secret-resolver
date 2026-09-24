// Package serve runs Mamori providers as stdio RPC servers for mamori-resolver.
package serve

import (
	"context"
	"fmt"
	"io"
	"net/rpc"
	"os"
	"runtime/debug"

	"github.com/xavidop/mamori"
	mrpc "github.com/yaronf/mamori-resolver/rpc"
)

// Options configure Serve.
type Options struct {
	Name    string // Info.ProviderName
	Version string // Info.ProviderVersion; default: build info
	Stdin   io.ReadCloser
	Stdout  io.WriteCloser
}

// Serve exposes providers over framed gob net/rpc on stdin/stdout.
// stderr is left for diagnostics. Does not blank-import or use mamori.Register;
// pass provider instances explicitly (e.g. sqlite.New()).
func Serve(providers ...mamori.Provider) error {
	return ServeWith(Options{}, providers...)
}

// ServeWith is Serve with explicit IO / naming.
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
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}

	svc := &service{
		name:    opts.Name,
		version: opts.Version,
		byScheme: byScheme,
		schemes: schemes,
	}
	srv := rpc.NewServer()
	if err := srv.RegisterName(mrpc.ServiceName, svc); err != nil {
		return err
	}
	conn := mrpc.NewStdioConn(opts.Stdin, opts.Stdout)
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
	if args == nil || args.URI == "" {
		reply.Err = &mrpc.RPCError{Kind: string(mamori.KindInvalid), Message: "empty URI"}
		return nil
	}
	ref, err := mamori.ParseRef(args.URI)
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
	// Never put secret material in Message; providers already avoid that.
	msg := err.Error()
	// Cap length to reduce accidental leakage of large payloads in errors.
	const max = 512
	if len(msg) > max {
		msg = msg[:max] + "…"
	}
	return &mrpc.RPCError{Kind: string(kind), Message: msg}
}
