package secretresolver

import "time"

// Value is the resolved secret/config payload, mirroring mamori.Value so
// callers get the same metadata without depending on Mamori core.
type Value struct {
	Bytes     []byte
	Version   string
	Sensitive bool
	NotAfter  time.Time
	Metadata  map[string]string
}
