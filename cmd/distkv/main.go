package main

import (
	"distkv/server"
	"distkv/store"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

func check(ok bool, msg string) {
	if ok {
		fmt.Println("PASS:", msg)
	} else {
		fmt.Println("FAIL:", msg)
	}
}

func waitForServer() {
	for {
		conn, err := net.Dial("tcp", "localhost:8080")
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func tcpSend(method, key, val string) string {
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		return ""
	}
	defer conn.Close()

	var req string
	if method == "PUT" {
		req = fmt.Sprintf("PUT|/%s|%s", key, val)
	} else if method == "DELETE" {
		req = fmt.Sprintf("DELETE|/%s", key)
	} else {
		req = fmt.Sprintf("GET|/%s", key)
	}

	conn.Write([]byte(req))

	buf := make([]byte, 4096)
	n, _ := conn.Read(buf)
	return strings.TrimRight(string(buf[:n]), "\n")
}

func tcpGet(key string) string {
	return tcpSend("GET", key, "")
}

func tcpPut(key, val string) bool {
	resp := tcpSend("PUT", key, val)
	return resp != ""
}

func tcpDelete(key string) bool {
	resp := tcpSend("DELETE", key, "")
	return resp != ""
}

func testBasics() {
	fmt.Println("\n=== Basic TCP Tests ===")

	ok := tcpPut("color", "blue")
	check(ok, "PUT returns success")

	val := tcpGet("color")
	check(val == "blue", "GET returns correct value")

	val = tcpGet("nonexistent")
	check(val == "", "GET missing key returns empty")

	ok = tcpPut("color", "red")
	check(ok, "PUT overwrite returns success")

	val = tcpGet("color")
	check(val == "red", "GET returns overwritten value")

	ok = tcpDelete("color")
	check(ok, "DELETE returns success")

	val = tcpGet("color")
	check(val == "", "GET after delete returns empty")

	ok = tcpDelete("still_nonexistent")
	check(ok, "DELETE non-existent returns success")
}

func testConcurrentAccess() {
	fmt.Println("\n=== Concurrent Stress Test (TCP) ===")

	var wg sync.WaitGroup
	var mu sync.Mutex
	failures := 0

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", n%10)
			val := fmt.Sprintf("val%d", n)

			ok := tcpPut(key, val)
			if !ok {
				mu.Lock()
				failures++
				mu.Unlock()
				return
			}

			got := tcpGet(key)
			if got == "" {
				mu.Lock()
				failures++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	check(failures == 0, fmt.Sprintf("Concurrent stress test (0 failures, got %d)", failures))
}

func testSaveFailure() {
	fmt.Println("\n=== Save Failure Test ===")

	os.Chmod("snapshot", 0444)
	defer os.Chmod("snapshot", 0755)

	ok := tcpPut("will-fail", "x")
	check(!ok, "PUT fails when disk is unwritable")

	ok = tcpDelete("will-fail")
	check(!ok, "DELETE fails when disk is unwritable")
}

func testCorruptedSnapshot() {
	fmt.Println("\n=== Corrupted Snapshot Test ===")

	os.WriteFile("snapshot/corrupt.json", []byte("{{{not json}}}"), 0644)
	s := store.NewStore("snapshot/corrupt.json")
	_, ok := s.Get("anything")
	check(!ok, "Store starts empty after corrupt snapshot load")
	os.Remove("snapshot/corrupt.json")
}

func main() {
	go server.RunServer("server/config-test.json", "snapshot/data.json")

	waitForServer()

	testBasics()
	testConcurrentAccess()
	testSaveFailure()
	testCorruptedSnapshot()

	fmt.Println("\nAll TCP tests completed.")
}
