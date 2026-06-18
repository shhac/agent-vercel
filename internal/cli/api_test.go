package cli

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

func TestAPICallNonGetWithBodyReachesServer(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// The raw escape hatch must pass --body through to a state-changing call and
	// return the response. The env POST fixture echoes the posted body.
	out, _, err := execCLI(t, srv.URL, "api", "call", "POST", "/v10/projects/web/env",
		"--body", `{"key":"K","value":"v","type":"plain","target":["production"]}`, "--yes")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	m := decodeJSON(t, out)
	created, ok := m["created"].(map[string]any)
	if !ok || created["key"] != "K" || created["value"] != "v" {
		t.Fatalf("api call body did not round-trip: %v", m)
	}
}

func TestReadAPIBody(t *testing.T) {
	// empty → nil payload (no body sent)
	if v, err := readAPIBody(""); err != nil || v != nil {
		t.Fatalf("readAPIBody(\"\") = %v, %v; want nil, nil", v, err)
	}

	// inline JSON → RawMessage passed verbatim
	v, err := readAPIBody(`{"a":1}`)
	if err != nil {
		t.Fatalf("inline: %v", err)
	}
	if rm, ok := v.(json.RawMessage); !ok || string(rm) != `{"a":1}` {
		t.Fatalf("inline = %#v; want RawMessage {\"a\":1}", v)
	}

	// "-" → read from stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = orig }()
	go func() {
		_, _ = io.WriteString(w, `{"b":2}`)
		_ = w.Close()
	}()
	v, err = readAPIBody("-")
	if err != nil {
		t.Fatalf("stdin: %v", err)
	}
	if rm, ok := v.(json.RawMessage); !ok || string(rm) != `{"b":2}` {
		t.Fatalf("stdin = %#v; want RawMessage {\"b\":2}", v)
	}
}
