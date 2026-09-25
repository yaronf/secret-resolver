package rpc

import "time"

// ProtocolVersion is the wire shape version. Bump when request/response change.
const ProtocolVersion uint32 = 1

// ServiceName is the net/rpc service name (methods: Info, Resolve).
// Kept as "Mamori": this is the Mamori provider wire protocol.
const ServiceName = "Mamori"

type InfoRequest struct{}

type InfoResponse struct {
	ProtocolVersion uint32
	ProviderName    string
	ProviderVersion string
	Schemes         []string
}

type ResolveRequest struct {
	Ref      string // mamori source ref (not a standards-track URI)
	Deadline *time.Time
}

type ResolveResponse struct {
	Bytes     []byte
	Version   string
	Sensitive bool
	NotAfter  time.Time
	Metadata  map[string]string
}

// WireError is returned as a net/rpc error body (gob-encoded via rpc.ServerError
// is insufficient for Kind). We embed Kind in the error string as
// "kind\x1fmessage" for a stdlib-compatible path, and also support a structured
// reply field — Resolve returns error via a dedicated Result envelope.
type ResolveResult struct {
	OK    bool
	Value ResolveResponse
	Err   *RPCError
}

type RPCError struct {
	Kind    string
	Message string
}

func (e *RPCError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return e.Kind
	}
	return e.Kind + ": " + e.Message
}
