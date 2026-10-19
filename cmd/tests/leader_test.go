package tests

import (
	"strings"
	"testing"

	"distkv/server"
)

func TestLeaderBasicOperations(t *testing.T) {
	addr := startPrimary(t, server.Config{Role: "primary", Addr: freePort(t)})

	if resp := tcpPut(t, addr, "/color", "blue"); !strings.Contains(resp, "succesfully set") {
		t.Fatalf("PUT response = %q", resp)
	}
	if v := tcpGet(t, addr, "/color"); v != "blue" {
		t.Fatalf("GET = %q, want blue", v)
	}
	if resp := tcpPut(t, addr, "/color", "red"); !strings.Contains(resp, "succesfully set") {
		t.Fatalf("overwrite response = %q", resp)
	}
	if v := tcpGet(t, addr, "/color"); v != "red" {
		t.Fatalf("GET after overwrite = %q, want red", v)
	}
	if resp := tcpDelete(t, addr, "/color"); !strings.Contains(resp, "succesfully deleted") {
		t.Fatalf("DELETE response = %q", resp)
	}
	if v := tcpGet(t, addr, "/color"); v != "" {
		t.Fatalf("GET after delete = %q, want empty", v)
	}
}

func TestLeaderGetMissingReturnsEmpty(t *testing.T) {
	addr := startPrimary(t, server.Config{Role: "primary", Addr: freePort(t)})
	if v := tcpGet(t, addr, "/nope"); v != "" {
		t.Fatalf("GET missing = %q, want empty", v)
	}
}

func TestLeaderMalformedRequest(t *testing.T) {
	addr := startPrimary(t, server.Config{Role: "primary", Addr: freePort(t)})
	resp := tcpSend(t, addr, "GET\n")
	if !strings.Contains(resp, "Invalid request") {
		t.Fatalf("malformed response = %q", resp)
	}
}

func TestLeaderPutMissingValue(t *testing.T) {
	addr := startPrimary(t, server.Config{Role: "primary", Addr: freePort(t)})
	resp := tcpSend(t, addr, "PUT|/k\n")
	if !strings.Contains(resp, "missing value") {
		t.Fatalf("PUT missing value response = %q", resp)
	}
}

func TestLeaderSyncEqualVersion(t *testing.T) {
	addr := startPrimary(t, server.Config{Role: "primary", Addr: freePort(t)})
	resp := tcpSend(t, addr, "SYNC|0\n")
	if !strings.HasPrefix(resp, "SYNC_OK") {
		t.Fatalf("SYNC equal version response = %q", resp)
	}
}

func TestLeaderSyncMismatchReturnsData(t *testing.T) {
	addr := startPrimary(t, server.Config{Role: "primary", Addr: freePort(t)})
	tcpPut(t, addr, "/color", "blue")

	resp := tcpSend(t, addr, "SYNC|99\n")
	if !strings.HasPrefix(resp, "SYNC_DATA|") {
		t.Fatalf("SYNC mismatch response = %q", resp)
	}
	if !strings.Contains(resp, "/color") || !strings.Contains(resp, "blue") {
		t.Fatalf("SYNC data missing payload: %q", resp)
	}
}

func TestLeaderSyncInvalidVersion(t *testing.T) {
	addr := startPrimary(t, server.Config{Role: "primary", Addr: freePort(t)})
	resp := tcpSend(t, addr, "SYNC|notanumber\n")
	if !strings.Contains(resp, "invalid SYNC version") {
		t.Fatalf("invalid version response = %q", resp)
	}
}
