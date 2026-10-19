package tests

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	"distkv/server"
)

func TestEncoder(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain", "plain"},
		{"a|b", `a\|b`},
		{`a\b`, `a\\b`},
		{"a\nb", `a\nb`},
		{`|` + `\`, `\|\\`},
	}
	for _, c := range cases {
		if got := server.Encoder(c.in); got != c.want {
			t.Errorf("Encoder(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDecodeRequest(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"GET|/color\n", []string{"GET", "/color"}},
		{"SYNC|5\n", []string{"SYNC", "5"}},
		{"GET\n", []string{"GET"}},
		{`PUT|k\|1|v` + "\n", []string{"PUT", "k|1", "v"}},
		{`PUT|k|a\nb` + "\n", []string{"PUT", "k", "a\nb"}},
		{`a\\b`, []string{`a\b`}},
		{"a\\", []string{"a\\"}},
		{`a\x`, []string{`a\x`}},
	}
	for _, c := range cases {
		got := server.DecodeRequest(c.in)
		if len(got) != len(c.want) {
			t.Errorf("DecodeRequest(%q) = %q, want %q", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("DecodeRequest(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	cases := []struct{ method, key, val string }{
		{"PUT", "k|1", "v\n2"},
		{"PUT", `back\slash`, "tab\tvalue"},
		{"GET", "weird|key\\)", ""},
	}
	for _, c := range cases {
		req := server.EncodeRequest(c.method, c.key, c.val)
		parts := server.DecodeRequest(req)
		if len(parts) < 2 || parts[0] != c.method || parts[1] != c.key {
			t.Fatalf("roundtrip method/key for %+v: got %q", c, parts)
		}
		if c.method == "PUT" {
			if len(parts) != 3 || parts[2] != c.val {
				t.Fatalf("roundtrip val for %+v: got %q", c, parts)
			}
		}
	}
}

func TestEncodeRequestFieldCount(t *testing.T) {
	if got := server.EncodeRequest("PUT", "k", "v"); got != "PUT|k|v\n" {
		t.Errorf("PUT frame = %q", got)
	}
	if got := server.EncodeRequest("GET", "k", ""); got != "GET|k\n" {
		t.Errorf("GET frame = %q", got)
	}
	if got := server.EncodeRequest("DELETE", "k", ""); got != "DELETE|k\n" {
		t.Errorf("DELETE frame = %q", got)
	}
}

func TestLoadConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg.json")
	if err := os.WriteFile(path, []byte(`{"role":"replica","address":":9000","peers":[":8081"],"leader":":8081"}`), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := server.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Role != "replica" || cfg.Addr != ":9000" || cfg.Leader != ":8081" || len(cfg.Peers) != 1 {
		t.Fatalf("unexpected config: %+v", cfg)
	}

	if _, err := server.LoadConfig(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Error("expected error for missing config")
	}

	bad := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(bad, []byte("{not json"), 0644)
	if _, err := server.LoadConfig(bad); err == nil {
		t.Error("expected error for malformed config")
	}
}

func TestForwardToPeer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	peer := ":" + port

	got := make(chan string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		b, _ := io.ReadAll(c)
		got <- string(b)
	}()

	if err := server.ForwardToPeer(peer, "PUT", "k", "v"); err != nil {
		t.Fatalf("ForwardToPeer: %v", err)
	}
	if g := <-got; g != "PUT|k|v\n" {
		t.Fatalf("peer received %q, want %q", g, "PUT|k|v\n")
	}
}
