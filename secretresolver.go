package secretresolver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/yaronf/secret-resolver/rpc"
)

// Provider is one out-of-process provider plugin. Configure via the API
// (WithProviders); a file format belongs in Mamori's own config if/when this
// lands upstream — not a parallel JSON schema here.
type Provider struct {
	Command string
	Args    []string
	Env     map[string]string
}

// Option configures New.
type Option func(*options)

type options struct {
	providers      []Provider
	stderr         io.Writer
	maxValueBytes  int
	maxConnRead    int64
}

// WithProviders registers provider child processes (required).
func WithProviders(ps ...Provider) Option {
	return func(o *options) { o.providers = append(o.providers, ps...) }
}

// WithStderr sets where provider stderr is forwarded (default os.Stderr).
func WithStderr(w io.Writer) Option {
	return func(o *options) { o.stderr = w }
}

// WithMaxValueBytes caps Resolve payload size (default rpc.DefaultMaxValueBytes).
func WithMaxValueBytes(n int) Option {
	return func(o *options) { o.maxValueBytes = n }
}

// Resolver routes URIs to out-of-process provider plugins.
type Resolver struct {
	mu            sync.RWMutex
	byScheme      map[string]*providerProc
	procs         []*providerProc
	stderr        io.Writer
	maxValueBytes int
}

type providerProc struct {
	cfg     Provider
	cmd     *exec.Cmd
	client  *rpc.Client
	info    *rpc.InfoResponse
	rpcFile *os.File // parent end of the RPC socketpair
}

// New starts configured provider processes and discovers their schemes.
func New(opts ...Option) (*Resolver, error) {
	o := options{
		stderr:        os.Stderr,
		maxValueBytes: rpc.DefaultMaxValueBytes,
		maxConnRead:   rpc.DefaultMaxConnRead,
	}
	for _, fn := range opts {
		fn(&o)
	}
	if len(o.providers) == 0 {
		return nil, fmt.Errorf("%w: no providers configured (use WithProviders)", ErrInvalid)
	}
	if o.maxValueBytes <= 0 {
		o.maxValueBytes = rpc.DefaultMaxValueBytes
	}

	r := &Resolver{
		byScheme:      make(map[string]*providerProc),
		stderr:        o.stderr,
		maxValueBytes: o.maxValueBytes,
	}
	for _, pcfg := range o.providers {
		p, err := r.startProvider(pcfg, o.maxConnRead)
		if err != nil {
			_ = r.Close()
			return nil, err
		}
		r.procs = append(r.procs, p)
		for _, scheme := range p.info.Schemes {
			if _, dup := r.byScheme[scheme]; dup {
				_ = r.Close()
				return nil, fmt.Errorf("%w: duplicate scheme %q", ErrInvalid, scheme)
			}
			r.byScheme[scheme] = p
		}
	}
	return r, nil
}

func (r *Resolver) startProvider(pcfg Provider, maxConnRead int64) (*providerProc, error) {
	if pcfg.Command == "" {
		return nil, fmt.Errorf("%w: empty provider command", ErrInvalid)
	}

	parent, child, err := rpc.NewSocketPair()
	if err != nil {
		return nil, err
	}

	env, err := buildChildEnv(pcfg.Env)
	if err != nil {
		_ = parent.Close()
		_ = child.Close()
		return nil, err
	}

	cmd := exec.Command(pcfg.Command, pcfg.Args...)
	cmd.Env = env
	// Leave stdout inherited so providers can log normally; forward stderr.
	cmd.Stderr = r.stderr
	cmd.ExtraFiles = []*os.File{child}

	if err := cmd.Start(); err != nil {
		_ = parent.Close()
		_ = child.Close()
		return nil, fmt.Errorf("start %s: %w", pcfg.Command, err)
	}
	_ = child.Close() // child process holds its own dup

	conn := rpc.LimitReads(parent, maxConnRead)
	client := rpc.NewTypedClient(rpc.NewClient(conn))

	ctx, cancel := rpc.PingDeadline()
	defer cancel()
	info, err := client.Info(ctx)
	if err != nil {
		_ = client.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return nil, fmt.Errorf("info %s: %w", pcfg.Command, err)
	}
	if info.ProtocolVersion != rpc.ProtocolVersion {
		_ = client.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return nil, fmt.Errorf("%w: %s protocol %d want %d", ErrInvalid, pcfg.Command, info.ProtocolVersion, rpc.ProtocolVersion)
	}
	if len(info.Schemes) == 0 {
		_ = client.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return nil, fmt.Errorf("%w: %s advertised no schemes", ErrInvalid, pcfg.Command)
	}

	return &providerProc{
		cfg:     pcfg,
		cmd:     cmd,
		client:  client,
		info:    info,
		rpcFile: parent,
	}, nil
}

// Resolve fetches uri via the provider that owns its scheme.
func (r *Resolver) Resolve(ctx context.Context, uri string) (Value, error) {
	ref, err := ParseRef(uri)
	if err != nil {
		return Value{}, err
	}
	r.mu.RLock()
	p := r.byScheme[ref.Scheme]
	maxBytes := r.maxValueBytes
	r.mu.RUnlock()
	if p == nil {
		return Value{}, &Error{Kind: KindInvalid, Message: fmt.Sprintf("no provider for scheme %q", ref.Scheme)}
	}

	res, err := p.client.Resolve(ctx, uri)
	if err != nil {
		if errors.Is(err, rpc.ErrTooLarge) {
			return Value{}, &Error{Kind: KindProtocol, Message: "provider RPC exceeded read budget"}
		}
		if p.cmd.ProcessState != nil && p.cmd.ProcessState.Exited() {
			return Value{}, &Error{Kind: KindProviderExit, Message: fmt.Sprintf("provider %s exited", p.cfg.Command)}
		}
		return Value{}, &Error{Kind: KindUnavailable, Message: err.Error()}
	}
	if res.Err != nil {
		return Value{}, &Error{
			Kind:    NormalizeKind(res.Err.Kind),
			Message: sanitizeErrorMessage(res.Err.Message),
		}
	}
	if !res.OK {
		return Value{}, &Error{Kind: KindUnknown, Message: "empty resolve result"}
	}
	if len(res.Value.Bytes) > maxBytes {
		return Value{}, &Error{
			Kind:    KindProtocol,
			Message: fmt.Sprintf("value exceeds max size (%d bytes)", maxBytes),
		}
	}
	return Value{
		Bytes:     res.Value.Bytes,
		Version:   res.Value.Version,
		Sensitive: res.Value.Sensitive,
		NotAfter:  res.Value.NotAfter,
		Metadata:  res.Value.Metadata,
	}, nil
}

// Close shuts down all provider processes.
func (r *Resolver) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var first error
	for _, p := range r.procs {
		if p.client != nil {
			if err := p.client.Close(); err != nil && first == nil {
				first = err
			}
		}
		if p.rpcFile != nil {
			_ = p.rpcFile.Close()
		}
		if p.cmd != nil && p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
			_, _ = p.cmd.Process.Wait()
		}
	}
	r.procs = nil
	r.byScheme = make(map[string]*providerProc)
	return first
}

// Schemes returns discovered schemes.
func (r *Resolver) Schemes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.byScheme))
	for s := range r.byScheme {
		out = append(out, s)
	}
	return out
}

// sanitizeErrorMessage keeps provider error text out of secret-shaped dumps:
// truncate and strip controls. Providers must still not put secret bytes in Error().
func sanitizeErrorMessage(msg string) string {
	const max = 512
	b := make([]byte, 0, min(len(msg), max+1))
	for i := 0; i < len(msg) && len(b) < max; i++ {
		c := msg[i]
		if c < 0x20 && c != '\t' || c == 0x7f {
			b = append(b, '?')
			continue
		}
		b = append(b, c)
	}
	if len(msg) > max {
		return string(b) + "…"
	}
	return string(b)
}
