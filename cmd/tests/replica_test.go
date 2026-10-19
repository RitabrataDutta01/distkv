package tests

import (
	"encoding/json"
	"net"
	"path/filepath"
	"strings"
	"testing"

	"distkv/server"
	"distkv/store"
)

func startStandaloneReplica(t *testing.T) string {
	t.Helper()
	cfg := server.Config{
		Role:   "replica",
		Addr:   freePort(t),
		Peers:  []string{},
		Leader: freePort(t),
	}
	return startReplica(t, cfg)
}

func TestReplicaPingPong(t *testing.T) {
	addr := startStandaloneReplica(t)
	if resp := tcpSend(t, addr, "PING|\n"); strings.TrimSpace(resp) != "PONG" {
		t.Fatalf("PING response = %q, want PONG", resp)
	}
}

func TestReplicaBasicOperations(t *testing.T) {
	addr := startStandaloneReplica(t)

	if resp := tcpPut(t, addr, "/animal", "dog"); !strings.Contains(resp, "succesfully set") {
		t.Fatalf("PUT response = %q", resp)
	}
	if v := tcpGet(t, addr, "/animal"); v != "dog" {
		t.Fatalf("GET = %q, want dog", v)
	}
	if resp := tcpDelete(t, addr, "/animal"); !strings.Contains(resp, "succesfully deleted") {
		t.Fatalf("DELETE response = %q", resp)
	}
	if v := tcpGet(t, addr, "/animal"); v != "" {
		t.Fatalf("GET after delete = %q, want empty", v)
	}
}

func TestReplicaOversizedPayload(t *testing.T) {
	addr := startStandaloneReplica(t)

	conn, err := net.Dial("tcp", dialAddr(addr))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	big := make([]byte, 2<<20)
	for i := range big {
		big[i] = 'a'
	}
	if _, err := conn.Write(big); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 128)
	n, _ := conn.Read(buf)
	if !strings.Contains(string(buf[:n]), "too big") {
		t.Fatalf("oversized response = %q", string(buf[:n]))
	}
}

func TestSyncWithLeaderAppliesSnapshot(t *testing.T) {
	payload := `{"version":7,"timestamp":123,"data":{"/color":"blue"}}`

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	_, port, _ := net.SplitHostPort(ln.Addr().String())

	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		req := make([]byte, 256)
		c.Read(req)
		c.Write([]byte("SYNC_DATA|" + payload + "\n"))
	}()

	st := store.NewStore(filepath.Join(t.TempDir(), "snap.json"))
	server.SyncWithLeader(":"+port, st)

	if v, ok := st.Get("/color"); !ok || v != "blue" {
		t.Fatalf("after sync Get = (%q,%v), want (blue,true)", v, ok)
	}
	if st.Version() != 7 {
		t.Fatalf("after sync version = %d, want 7", st.Version())
	}

	var ff struct {
		Version int `json:"version"`
	}
	// sanity: payload is valid JSON of the expected shape
	if err := json.Unmarshal([]byte(payload), &ff); err != nil || ff.Version != 7 {
		t.Fatalf("payload sanity failed: %v", err)
	}
}

func TestSyncWithLeaderEqualVersionNoop(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	_, port, _ := net.SplitHostPort(ln.Addr().String())

	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		req := make([]byte, 256)
		c.Read(req)
		c.Write([]byte("SYNC_OK\n"))
	}()

	path := filepath.Join(t.TempDir(), "snap.json")
	st := store.NewStore(path)
	st.Set("/keep", "me")

	server.SyncWithLeader(":"+port, st)

	if v, ok := st.Get("/keep"); !ok || v != "me" {
		t.Fatalf("SYNC_OK should not modify state, Get = (%q,%v)", v, ok)
	}
}
