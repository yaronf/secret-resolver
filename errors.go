package secretresolver

import (
	"errors"
	"fmt"
)

// Kind mirrors mamori.Kind for wire and caller classification, plus a few
// host-only kinds (protocol, provider_exit).
type Kind string

const (
	KindNotFound         Kind = "not_found"
	KindPermissionDenied Kind = "permission_denied"
	KindUnauthenticated  Kind = "unauthenticated"
	KindUnavailable      Kind = "unavailable"
	KindRateLimited      Kind = "rate_limited"
	KindInvalid          Kind = "invalid"
	KindUnknown          Kind = "unknown"
	KindProtocol         Kind = "protocol"
	KindProviderExit     Kind = "provider_exit"
)

var (
	ErrNotFound         = errors.New("secret-resolver: not found")
	ErrPermissionDenied = errors.New("secret-resolver: permission denied")
	ErrUnauthenticated  = errors.New("secret-resolver: unauthenticated")
	ErrUnavailable      = errors.New("secret-resolver: unavailable")
	ErrRateLimited      = errors.New("secret-resolver: rate limited")
	ErrInvalid          = errors.New("secret-resolver: invalid")
	ErrTooLarge         = errors.New("secret-resolver: payload too large")
)

// kindSentinels enumerates known kinds and the errors.Is sentinel for each
// (nil = no sentinel, same as mamori for unknown).
var kindSentinels = map[Kind]error{
	KindNotFound:         ErrNotFound,
	KindPermissionDenied: ErrPermissionDenied,
	KindUnauthenticated:  ErrUnauthenticated,
	KindUnavailable:      ErrUnavailable,
	KindRateLimited:      ErrRateLimited,
	KindInvalid:          ErrInvalid,
	KindUnknown:          nil,
	KindProtocol:         ErrTooLarge,
	KindProviderExit:     nil,
}

// NormalizeKind maps a wire kind string to a known Kind; unknown values become KindUnknown.
func NormalizeKind(s string) Kind {
	k := Kind(s)
	if _, ok := kindSentinels[k]; ok {
		return k
	}
	return KindUnknown
}

// Error is a classified resolve failure. Message must never contain secret bytes.
type Error struct {
	Kind    Kind
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return string(e.Kind)
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	sent, ok := kindSentinels[e.Kind]
	return ok && sent != nil && target == sent
}
