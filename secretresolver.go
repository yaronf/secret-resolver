package secretresolver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/yaronf/secret-resolver/rpc"
)

// Provider is one out-of-process provider plugin. Configure via the API
// (WithProviders); a file format belongs in Mamori's own config if/when this
// lands upstream.
type Provider struct {
	Command string
	Args    []string
	Env     map[string]string
}

// Option configures New.
type Option func(*options)

type options struct {
	providers     []Provider
	stdout        io.Writer
	stderr        io.Writer
	maxValueBytes int
	maxConnRead   int64
}

// WithProviders registers provider child processes (required).
func WithProviders(ps ...Provider) Option {
	return func(o *options) { o.providers = append(o.providers, ps...) }
}

// WithStdout sets where provider stdout is forwarded (default os.Stdout).
// RPC uses ExtraFiles, so stdout stays available for provider logging.
func WithStdout(w io.Writer) Option {
	return func(o *options) { o.stdout = w }
}

// WithStderr sets where provider stderr is forwarded (default os.Stderr).
func WithStderr(w io.Writer) Option {
	return func(o *options) { o.stderr = w }
}

// WithMaxValueBytes caps Resolve payload size (default rpc.DefaultMaxValueBytes).
func WithMaxValueBytes(n int) Option {
	return func(o *options) { o.maxValueBytes = n }
}

// Resolver routes refs to out-of-process provider plugins.
//
// The mutex guards byScheme/procs so Close is safe against concurrent Resolve.
// That same shape is a starting point for later dynamic provider load/unload;
// today the set is fixed at New. Close waits for in-flight Resolves before
// killing children.
type Resolver struct {
	mu            sync.RWMutex
	byScheme      map[string]*providerProc
	procs         []*providerProc
	stdout        io.Writer
	stderr        io.Writer
	maxValueBytes int

	closed   atomic.Bool
	inflight sync.WaitGroup
}

type providerProc struct {
	cfg    Provider
	cmd    *exec.Cmd
	client *rpc.Client
	info   *rpc.InfoResponse
}

// New starts configured provider processes and discovers their schemes.
func New(opts ...Option) (*Resolver, error) {
	o := options{
		stdout:        os.Stdout,
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
		stdout:        o.stdout,
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
	// RPC is on ExtraFiles; stdout/stderr are for provider logging / diagnostics.
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr
	cmd.ExtraFiles = []*os.File{child}

	if err := cmd.Start(); err != nil {
		_ = parent.Close()
		_ = child.Close()
		return nil, fmt.Errorf("start %s: %w", pcfg.Command, err)
	}
	_ = child.Close() // child process holds its own dup

	// Client owns parent (via LimitedConn); do not Close the file separately.
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
		cfg:    pcfg,
		cmd:    cmd,
		client: client,
		info:   info,
	}, nil
}

// Resolve fetches ref via the provider that owns its scheme.
func (r *Resolver) Resolve(ctx context.Context, ref string) (Value, error) {
	if r.closed.Load() {
		return Value{}, &Error{Kind: KindUnavailable, Message: "resolver closed", cause: ErrClosed}
	}

	parsed, err := ParseRef(ref)
	if err != nil {
		return Value{}, err
	}

	r.mu.RLock()
	if r.closed.Load() {
		r.mu.RUnlock()
		return Value{}, &Error{Kind: KindUnavailable, Message: "resolver closed", cause: ErrClosed}
	}
	p := r.byScheme[parsed.Scheme]
	maxBytes := r.maxValueBytes
	r.mu.RUnlock()
	if p == nil {
		return Value{}, &Error{Kind: KindInvalid, Message: fmt.Sprintf("no provider for scheme %q", parsed.Scheme)}
	}

	r.inflight.Add(1)
	defer r.inflight.Done()

	res, err := p.client.Resolve(ctx, ref)
	if err != nil {
		if errors.Is(err, rpc.ErrTooLarge) {
			return Value{}, &Error{
				Kind:    KindProtocol,
				Message: "provider RPC exceeded read budget",
				cause:   ErrTooLarge,
			}
		}
		if p.cmd.ProcessState != nil && p.cmd.ProcessState.Exited() {
			return Value{}, &Error{Kind: KindProviderExit, Message: fmt.Sprintf("provider %s exited", p.cfg.Command)}
		}
		return Value{}, &Error{Kind: KindUnavailable, Message: err.Error()}
	}
	if res.Err != nil {
		kind := NormalizeKind(res.Err.Kind)
		out := &Error{Kind: kind, Message: truncateMessage(res.Err.Message)}
		if kind == KindProtocol {
			out.cause = ErrTooLarge // wire "protocol" today means size/budget
		}
		return Value{}, out
	}
	if !res.OK {
		return Value{}, &Error{Kind: KindUnknown, Message: "empty resolve result"}
	}
	if len(res.Value.Bytes) > maxBytes {
		return Value{}, &Error{
			Kind:    KindProtocol,
			Message: fmt.Sprintf("value exceeds max size (%d bytes)", maxBytes),
			cause:   ErrTooLarge,
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

// Close shuts down all provider processes. It rejects new Resolves, waits for
// in-flight ones to finish, then kills children. Concurrent Close is safe.
func (r *Resolver) Close() error {
	r.closed.Store(true)

	r.mu.Lock()
	procs := r.procs
	r.procs = nil
	r.byScheme = make(map[string]*providerProc)
	r.mu.Unlock()

	r.inflight.Wait()

	var first error
	for _, p := range procs {
		if p.client != nil {
			if err := p.client.Close(); err != nil && first == nil {
				first = err
			}
		}
		if p.cmd != nil && p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
			_, _ = p.cmd.Process.Wait()
		}
	}
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

// truncateMessage bounds provider error text. Providers must still not put
// secret bytes in Error(); this only limits how much we retain.
func truncateMessage(msg string) string {
	const max = 512
	if len(msg) <= max {
		return msg
	}
	return msg[:max] + "…"
}
