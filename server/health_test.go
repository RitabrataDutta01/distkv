package server

import (
	"bufio"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"
)

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	defer l.Close()
	_, port, _ := net.SplitHostPort(l.Addr().String())
	return ":" + port
}

func startPongServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				bufio.NewReader(c).ReadString('\n')
				c.Write([]byte("PONG\n"))
			}(c)
		}
	}()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	return ":" + port
}

func startCaptureServer(t *testing.T) (string, <-chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	ch := make(chan string, 8)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				b, _ := io.ReadAll(c)
				c.Close()
				ch <- string(b)
			}(c)
		}
	}()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	return ":" + port, ch
}

func TestHealthCheck(t *testing.T) {
	alive := startPongServer(t)
	dead := freePort(t)

	healthy, deadPeers := healthCheck([]string{alive, dead})

	found := func(list []string, want string) bool {
		for _, s := range list {
			if s == want {
				return true
			}
		}
		return false
	}
	if !found(healthy, alive) {
		t.Errorf("expected %s healthy, got healthy=%v dead=%v", alive, healthy, deadPeers)
	}
	if !found(deadPeers, dead) {
		t.Errorf("expected %s dead, got healthy=%v dead=%v", dead, healthy, deadPeers)
	}
}

func TestWriteReg(t *testing.T) {
	reg := &peerRegistry{alive: map[string]bool{}}
	writeReg(reg, []string{":1"}, []string{":2"})

	if !reg.alive[":1"] {
		t.Error("expected :1 alive")
	}
	if reg.alive[":2"] {
		t.Error("expected :2 dead")
	}
}

func TestForwardToAllPeersDeliversAndReportsFailures(t *testing.T) {
	live, ch := startCaptureServer(t)
	dead := freePort(t)

	reg := &peerRegistry{alive: map[string]bool{live: true, dead: true}}
	failed := ForwardToAllPeers("PUT", "k", "v", []string{live, dead}, reg)

	if len(failed) != 1 || failed[0] != dead {
		t.Fatalf("failed peers = %v, want [%s]", failed, dead)
	}
	select {
	case got := <-ch:
		if got != "PUT|k|v\n" {
			t.Fatalf("live peer received %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("live peer received nothing")
	}
}

func TestForwardToAllPeersSkipsDead(t *testing.T) {
	live, ch := startCaptureServer(t)
	reg := &peerRegistry{alive: map[string]bool{}}

	failed := ForwardToAllPeers("PUT", "k", "v", []string{live}, reg)
	if len(failed) != 0 {
		t.Fatalf("failed = %v, want none", failed)
	}
	select {
	case got := <-ch:
		t.Fatalf("dead registry peer should be skipped, but received %q", got)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestHealthCheckNonPong(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		bufio.NewReader(c).ReadString('\n')
		c.Write([]byte("NOPE\n"))
	}()
	_, port, _ := net.SplitHostPort(ln.Addr().String())

	healthy, dead := healthCheck([]string{":" + port})
	if len(healthy) != 0 || len(dead) != 1 {
		t.Fatalf("non-PONG peer: healthy=%v dead=%v", healthy, dead)
	}
}

func TestLeaderForwardFailureBranches(t *testing.T) {
	dead := freePort(t)
	reg := &peerRegistry{alive: map[string]bool{dead: true}}
	snap := LoadSnap(filepath.Join(t.TempDir(), "leader.json"))

	for _, req := range []string{"PUT|k|v\n", "DELETE|k\n"} {
		c1, c2 := net.Pipe()
		go snap.HandleLeaderConnection(c1, []string{dead}, reg)
		c2.Write([]byte(req))
		bufio.NewReader(c2).ReadString('\n')
		c2.Close()
	}
	time.Sleep(200 * time.Millisecond)
}

func TestLeaderReplicaPipeline(t *testing.T) {
	replSnap := LoadSnap(filepath.Join(t.TempDir(), "replica.json"))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go replSnap.HandleNodeConnection(c)
		}
	}()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	peer := ":" + port

	reg := &peerRegistry{alive: map[string]bool{peer: true}}
	leaderSnap := LoadSnap(filepath.Join(t.TempDir(), "leader.json"))

	c1, c2 := net.Pipe()
	go leaderSnap.HandleLeaderConnection(c1, []string{peer}, reg)

	if _, err := c2.Write([]byte("PUT|/color|blue\n")); err != nil {
		t.Fatal(err)
	}
	resp, _ := bufio.NewReader(c2).ReadString('\n')
	c2.Close()
	if resp == "" {
		t.Fatal("expected a response from leader")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if v, ok := replSnap.store.Get("/color"); ok && v == "blue" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("replica never received replicated value")
}
