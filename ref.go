package secretresolver

import (
	"fmt"
	"net/url"
	"strings"
)

// Ref is a parsed mamori-style source URI (scheme://path[#key][?opts]).
type Ref struct {
	Scheme string
	Path   string
	Key    string
	Opts   url.Values
	Raw    string
}

// ParseRef parses a mamori source tag. Fragment (#key) precedes query (?opts).
func ParseRef(tag string) (Ref, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return Ref{}, fmt.Errorf("%w: empty source ref", ErrInvalid)
	}
	scheme, remainder, ok := strings.Cut(tag, ":")
	if !ok || scheme == "" {
		return Ref{}, fmt.Errorf("%w: source ref %q missing scheme", ErrInvalid, tag)
	}
	ref := Ref{Scheme: scheme, Raw: tag, Opts: url.Values{}}
	rest := strings.TrimPrefix(remainder, "//")
	if i := strings.IndexByte(rest, '?'); i >= 0 {
		q, err := url.ParseQuery(rest[i+1:])
		if err != nil {
			return Ref{}, fmt.Errorf("%w: source ref %q bad query: %v", ErrInvalid, tag, err)
		}
		ref.Opts = q
		rest = rest[:i]
	}
	if i := strings.IndexByte(rest, '#'); i >= 0 {
		ref.Key = rest[i+1:]
		rest = rest[:i]
	}
	ref.Path = rest
	return ref, nil
}
