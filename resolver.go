package resolver

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/yaronf/mamori-resolver/config"
	"github.com/yaronf/mamori-resolver/rpc"
)

// Option configures New.
type Option func(*options)

type options struct {
	configPath string
	providers  []config.Provider
	stderr     io.Writer
}

// WithConfigFile loads providers from a JSON config file.
func WithConfigFile(path string) Option {
	return func(o *options) { o.configPath = path }
}

// WithProviders sets the provider list directly (tests / embedding).
func WithProviders(ps ...config.Provider) Option {
	return func(o *options) { o.providers = append(o.providers, ps...) }
}

// WithStderr sets where provider stderr is forwarded (default os.Stderr).
func WithStderr(w io.Writer) Option {
	return func(o *options) { o.stderr = w }
}

// Resolver routes URIs to out-of-process provider plugins.
type Resolver struct {
	mu       sync.RWMutex
	byScheme map[string]*providerProc
	procs    []*providerProc
	stderr   io.Writer
}

type providerProc struct {
	cfg    config.Provider
	cmd    *exec.Cmd
	client *rpc.Client
	info   *rpc.InfoResponse
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

// New starts configured provider processes and discovers their schemes.
func New(opts ...Option) (*Resolver, error) {
	o := options{stderr: os.Stderr}
	for _, fn := range opts {
		fn(&o)
	}
	providers := o.providers
	if o.configPath != "" {
		f, err := config.LoadJSON(o.configPath)
		if err != nil {
			return nil, err
		}
		providers = append(providers, f.Providers...)
	}
	if len(providers) == 0 {
		return nil, fmt.Errorf("%w: no providers configured", ErrInvalid)
	}

	r := &Resolver{
		byScheme: make(map[string]*providerProc),
		stderr:   o.stderr,
	}
	for _, pcfg := range providers {
		p, err := r.startProvider(pcfg)
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

func (r *Resolver) startProvider(pcfg config.Provider) (*providerProc, error) {
	cmd := exec.Command(pcfg.Command, pcfg.Args...)
	cmd.Env = os.Environ()
	for k, v := range pcfg.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Stderr = r.stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("start %s: %w", pcfg.Command, err)
	}

	conn := rpc.NewStdioConn(stdout, stdin)
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
		stdin:  stdin,
		stdout: stdout,
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
	r.mu.RUnlock()
	if p == nil {
		return Value{}, &Error{Kind: KindInvalid, Message: fmt.Sprintf("no provider for scheme %q", ref.Scheme)}
	}

	res, err := p.client.Resolve(ctx, uri)
	if err != nil {
		if p.cmd.ProcessState != nil && p.cmd.ProcessState.Exited() {
			return Value{}, &Error{Kind: KindProviderExit, Message: fmt.Sprintf("provider %s exited", p.cfg.Command)}
		}
		return Value{}, &Error{Kind: KindUnavailable, Message: err.Error()}
	}
	if res.Err != nil {
		return Value{}, &Error{Kind: Kind(res.Err.Kind), Message: res.Err.Message}
	}
	if !res.OK {
		return Value{}, &Error{Kind: KindUnknown, Message: "empty resolve result"}
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
