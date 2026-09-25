package main

import (
	"testing"

	secretresolver "github.com/yaronf/secret-resolver"
)

func TestFormatResolveJSONRedactsByDefault(t *testing.T) {
	v := secretresolver.Value{Bytes: []byte("hunter2"), Version: "1", Sensitive: true}
	out := formatResolveJSON(v, false)
	if out.Bytes != "" || !out.Redacted || out.ByteLen != 7 {
		t.Fatalf("%+v", out)
	}
	shown := formatResolveJSON(v, true)
	if shown.Bytes != "hunter2" || shown.Redacted {
		t.Fatalf("%+v", shown)
	}
}
