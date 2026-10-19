package tests

import (
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"distkv/server"
)

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	defer l.Close()
	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		t.Fatalf("freePort split: %v", err)
	}
	return ":" + port
}

func dialAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	return addr
}

func writeConfig(t *testing.T, cfg server.Config) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func waitForDial(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.Dial("tcp", dialAddr(addr))
		if err == nil {
			c.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server at %s did not come up", addr)
}

func startPrimary(t *testing.T, cfg server.Config) string {
	t.Helper()
	cfgPath := writeConfig(t, cfg)
	snapPath := filepath.Join(t.TempDir(), "snapshot.json")
	go server.RunServer(cfgPath, snapPath)
	waitForDial(t, cfg.Addr)
	return cfg.Addr
}

func startReplica(t *testing.T, cfg server.Config) string {
	t.Helper()
	cfgPath := writeConfig(t, cfg)
	snapPath := filepath.Join(t.TempDir(), "snapshot.json")
	go server.RunNode(cfgPath, snapPath)
	waitForDial(t, cfg.Addr)
	return cfg.Addr
}

func tcpSend(t *testing.T, addr, req string) string {
	t.Helper()
	conn, err := net.Dial("tcp", dialAddr(addr))
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatalf("write: %v", err)
	}
	b, _ := io.ReadAll(conn)
	return string(b)
}

func tcpGet(t *testing.T, addr, key string) string {
	return tcpSend(t, addr, server.EncodeRequest("GET", key, ""))
}

func tcpPut(t *testing.T, addr, key, val string) string {
	return tcpSend(t, addr, server.EncodeRequest("PUT", key, val))
}

func tcpDelete(t *testing.T, addr, key string) string {
	return tcpSend(t, addr, server.EncodeRequest("DELETE", key, ""))
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cond()
}
