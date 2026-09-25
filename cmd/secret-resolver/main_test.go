package main

import (
	"testing"

	secretresolver "github.com/yaronf/secret-resolver"
)

func TestFormatResolveJSONRedactsByDefault(t *testing.T) {
	v := secretresolver.Value{
		Bytes:     []byte("hunter2"),
		Version:   "1",
		Sensitive: true,
		Metadata:  map[string]string{"hint": "secret-ish"},
	}
	out := formatResolveJSON(v, false)
	if out.Bytes != "*******" || !out.Redacted || out.ByteLen != 7 || out.Metadata != nil {
		t.Fatalf("%+v", out)
	}
	huge := secretresolver.Value{Bytes: make([]byte, 100), Version: "1"}
	out = formatResolveJSON(huge, false)
	if out.Bytes != "************" || out.ByteLen != 100 {
		t.Fatalf("capped stars: %+v", out)
	}
	shown := formatResolveJSON(v, true)
	if shown.Bytes != "hunter2" || shown.Redacted || shown.Metadata["hint"] != "secret-ish" {
		t.Fatalf("%+v", shown)
	}
}
