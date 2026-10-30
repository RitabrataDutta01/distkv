package tests

import (
	"distkv/server"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func waitForServer(t *testing.T, addr string) {
	t.Helper()
	for i := 0; i < 50; i++ {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("Server at %s failed to start", addr)
}

func tcpSend(t *testing.T, addr, method, key, val string) string {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to dial %s: %v", addr, err)
	}
	defer conn.Close()

	req := server.EncodeRequest(method, "/"+key, val)
	conn.Write([]byte(req))

	buf := make([]byte, 4096)
	n, _ := conn.Read(buf)
	return strings.TrimRight(string(buf[:n]), "\n")
}

func TestTCPIntegration(t *testing.T) {
	tempDir := t.TempDir()
	snapPath := filepath.Join(tempDir, "data.json")
	cfgPath := filepath.Join(tempDir, "config.json")

	os.WriteFile(cfgPath, []byte(`{"role":"primary", "address":":8080"}`), 0644)

	go server.RunServer(cfgPath, snapPath)
	waitForServer(t, "localhost:8080")

	t.Run("Basics", func(t *testing.T) {
		resp := tcpSend(t, "localhost:8080", "PUT", "color", "blue")
		if resp == "" {
			t.Fatal("PUT failed")
		}

		val := tcpSend(t, "localhost:8080", "GET", "color", "")
		if val != "blue" {
			t.Fatalf("GET expected 'blue', got %q", val)
		}

		tcpSend(t, "localhost:8080", "DELETE", "color", "")
		val = tcpSend(t, "localhost:8080", "GET", "color", "")
		if val != "" {
			t.Fatalf("GET after delete expected empty, got %q", val)
		}
	})

	t.Run("Escaping", func(t *testing.T) {
		tcpSend(t, "localhost:8080", "PUT", "tricky", "a|b\nc")
		val := tcpSend(t, "localhost:8080", "GET", "tricky", "")
		if val != "a|b\nc" {
			t.Fatalf("Escaping failed, got %q", val)
		}
	})

	t.Run("Concurrency", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				key := fmt.Sprintf("key%d", n%10)
				val := fmt.Sprintf("val%d", n)

				tcpSend(t, "localhost:8080", "PUT", key, val)
				if got := tcpSend(t, "localhost:8080", "GET", key, ""); got == "" {
					t.Errorf("Concurrent GET returned empty for %s", key)
				}
			}(i)
		}
		wg.Wait()
	})
}
