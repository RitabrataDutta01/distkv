package tests

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"distkv/server"
	"distkv/store"
)

func TestApplySnapshotInvalidJSON(t *testing.T) {
	s := store.NewStore(filepath.Join(t.TempDir(), "snap.json"))
	if err := s.ApplySnapshot([]byte("{not valid json")); err == nil {
		t.Fatal("expected error for invalid snapshot JSON")
	}
}

func TestLegacySnapshotNonStringValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snap.json")
	if err := os.WriteFile(path, []byte(`{"a":1}`), 0644); err != nil {
		t.Fatal(err)
	}
	s := store.NewStore(path)
	if _, ok := s.Get("a"); ok {
		t.Fatal("expected empty store for invalid legacy snapshot")
	}
}

func startReplicaWithSnapshot(t *testing.T, cfg server.Config) (string, string) {
	t.Helper()
	cfgPath := writeConfig(t, cfg)
	snapPath := filepath.Join(t.TempDir(), "snapshot.json")
	go server.RunNode(cfgPath, snapPath)
	waitForDial(t, cfg.Addr)
	return cfg.Addr, snapPath
}

func TestReplicaMalformedRequest(t *testing.T) {
	addr := startStandaloneReplica(t)
	if resp := tcpSend(t, addr, "GET\n"); resp != "" {
		t.Fatalf("malformed request response = %q, want empty", resp)
	}
}

func TestReplicaPutMissingValue(t *testing.T) {
	addr := startStandaloneReplica(t)
	if resp := tcpSend(t, addr, "PUT|/k\n"); resp != "" {
		t.Fatalf("PUT missing value response = %q, want empty", resp)
	}
}

func TestReplicaSaveFailure(t *testing.T) {
	addr, snapPath := startReplicaWithSnapshot(t, server.Config{
		Role:   "replica",
		Addr:   freePort(t),
		Leader: freePort(t),
	})
	dir := filepath.Dir(snapPath)
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0755)

	if resp := tcpPut(t, addr, "/k", "v"); resp != "" {
		t.Fatalf("PUT on unwritable disk response = %q, want empty", resp)
	}
	if resp := tcpDelete(t, addr, "/k"); resp != "" {
		t.Fatalf("DELETE on unwritable disk response = %q, want empty", resp)
	}
}

func TestSyncWithLeaderReadError(t *testing.T) {
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
		c.Close()
	}()

	st := store.NewStore(filepath.Join(t.TempDir(), "snap.json"))
	server.SyncWithLeader(":"+port, st)
	if _, ok := st.Get("anything"); ok {
		t.Fatal("store should be unchanged after read error")
	}
}

func TestSyncWithLeaderApplyError(t *testing.T) {
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
		buf := make([]byte, 128)
		c.Read(buf)
		c.Write([]byte("SYNC_DATA|not-json\n"))
	}()

	st := store.NewStore(filepath.Join(t.TempDir(), "snap.json"))
	server.SyncWithLeader(":"+port, st)
	if _, ok := st.Get("anything"); ok {
		t.Fatal("store should be unchanged after invalid snapshot")
	}
}

func TestRunServerBadConfig(t *testing.T) {
	server.RunServer(filepath.Join(t.TempDir(), "missing.json"), filepath.Join(t.TempDir(), "snap.json"))
}

func TestRunNodeBadConfig(t *testing.T) {
	server.RunNode(filepath.Join(t.TempDir(), "missing.json"), filepath.Join(t.TempDir(), "snap.json"))
}

func TestRunServerListenError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	cfgPath := writeConfig(t, server.Config{Role: "primary", Addr: ln.Addr().String()})
	server.RunServer(cfgPath, filepath.Join(t.TempDir(), "snap.json"))
}

func TestRunNodeListenError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	cfgPath := writeConfig(t, server.Config{
		Role:   "replica",
		Addr:   ln.Addr().String(),
		Leader: freePort(t),
	})
	server.RunNode(cfgPath, filepath.Join(t.TempDir(), "snap.json"))
}
