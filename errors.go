package resolver

import (
	"errors"
	"fmt"
)

// Kind mirrors mamori.Kind for wire and caller classification.
type Kind string

const (
	KindNotFound          Kind = "not_found"
	KindPermissionDenied  Kind = "permission_denied"
	KindUnauthenticated   Kind = "unauthenticated"
	KindUnavailable       Kind = "unavailable"
	KindRateLimited       Kind = "rate_limited"
	KindInvalid           Kind = "invalid"
	KindUnknown           Kind = "unknown"
	KindProtocol          Kind = "protocol"
	KindProviderExit      Kind = "provider_exit"
)

var (
	ErrNotFound         = errors.New("mamori-resolver: not found")
	ErrPermissionDenied = errors.New("mamori-resolver: permission denied")
	ErrUnauthenticated  = errors.New("mamori-resolver: unauthenticated")
	ErrUnavailable      = errors.New("mamori-resolver: unavailable")
	ErrRateLimited      = errors.New("mamori-resolver: rate limited")
	ErrInvalid          = errors.New("mamori-resolver: invalid")
)

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
	switch e.Kind {
	case KindNotFound:
		return target == ErrNotFound
	case KindPermissionDenied:
		return target == ErrPermissionDenied
	case KindUnauthenticated:
		return target == ErrUnauthenticated
	case KindUnavailable:
		return target == ErrUnavailable
	case KindRateLimited:
		return target == ErrRateLimited
	case KindInvalid:
		return target == ErrInvalid
	}
	return false
}
