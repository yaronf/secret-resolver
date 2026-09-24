// Command mamori-provider-fake is an in-memory provider for resolver tests.
// Env FAKE_VALUES is JSON object map[string]string of full URI -> value.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/rpc"
	"os"
	"strings"
	"sync"

	mrpc "github.com/yaronf/mamori-resolver/rpc"
)

func main() {
	values := map[string]string{}
	if s := os.Getenv("FAKE_VALUES"); s != "" {
		if err := json.Unmarshal([]byte(s), &values); err != nil {
			fmt.Fprintf(os.Stderr, "fake: FAKE_VALUES: %v\n", err)
			os.Exit(1)
		}
	}
	schemes := map[string]struct{}{}
	for uri := range values {
		if i := strings.IndexByte(uri, ':'); i > 0 {
			schemes[uri[:i]] = struct{}{}
		}
	}
	if len(schemes) == 0 {
		schemes["fake"] = struct{}{}
	}
	list := make([]string, 0, len(schemes))
	for s := range schemes {
		list = append(list, s)
	}

	svc := &fakeService{values: values, schemes: list}
	srv := rpc.NewServer()
	if err := srv.RegisterName(mrpc.ServiceName, svc); err != nil {
		fmt.Fprintf(os.Stderr, "fake: register: %v\n", err)
		os.Exit(1)
	}
	conn := mrpc.NewStdioConn(os.Stdin, os.Stdout)
	mrpc.ServeConn(srv, conn)
}

type fakeService struct {
	mu      sync.Mutex
	values  map[string]string
	schemes []string
}

func (s *fakeService) Info(_ *mrpc.InfoRequest, reply *mrpc.InfoResponse) error {
	*reply = mrpc.InfoResponse{
		ProtocolVersion: mrpc.ProtocolVersion,
		ProviderName:    "fake",
		ProviderVersion: "test",
		Schemes:         append([]string(nil), s.schemes...),
	}
	return nil
}

func (s *fakeService) Resolve(args *mrpc.ResolveRequest, reply *mrpc.ResolveResult) error {
	_ = context.Background()
	*reply = mrpc.ResolveResult{}
	s.mu.Lock()
	v, ok := s.values[args.URI]
	s.mu.Unlock()
	if !ok {
		reply.Err = &mrpc.RPCError{Kind: "not_found", Message: "missing " + args.URI}
		return nil
	}
	reply.OK = true
	reply.Value = mrpc.ResolveResponse{
		Bytes:   []byte(v),
		Version: "1",
	}
	return nil
}
