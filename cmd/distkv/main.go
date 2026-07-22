package main

import (
	"distkv/server"
	"fmt"
	"io"
	"net"
	"net/http"
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

func httpGet(key string) (string, int) {
	resp, err := http.Get("http://localhost:8080/" + key)
	if err != nil {
		return "", 0
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return strings.TrimRight(string(body), "\n"), resp.StatusCode
}

func httpPut(key, val string) int {
	req, _ := http.NewRequest("PUT", "http://localhost:8080/"+key, strings.NewReader(val))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	resp.Body.Close()
	return resp.StatusCode
}

func httpDelete(key string) int {
	req, _ := http.NewRequest("DELETE", "http://localhost:8080/"+key, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	resp.Body.Close()
	return resp.StatusCode
}

func testBasics() {
	fmt.Println("\n=== Basic HTTP Tests ===")

	code := httpPut("color", "blue")
	check(code == 200, "PUT returns 200")

	val, code := httpGet("color")
	check(val == "blue", "GET returns correct value")
	check(code == 200, "GET returns 200")

	_, code = httpGet("nonexistent")
	check(code == 404, "GET missing key returns 404")

	code = httpPut("color", "red")
	check(code == 200, "PUT overwrite returns 200")

	val, code = httpGet("color")
	check(val == "red", "GET returns overwritten value")
	check(code == 200, "GET overwritten returns 200")

	code = httpDelete("color")
	check(code == 200, "DELETE returns 200")

	_, code = httpGet("color")
	check(code == 404, "GET after delete returns 404")

	code = httpDelete("still_nonexistent")
	check(code == 200, "DELETE non-existent returns 200")
}

func testConcurrentAccess() {
	fmt.Println("\n=== Concurrent Stress Test (HTTP) ===")

	var wg sync.WaitGroup
	var mu sync.Mutex
	failures := 0

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", n%10)
			val := fmt.Sprintf("val%d", n)

			code := httpPut(key, val)
			if code != 200 {
				mu.Lock()
				failures++
				mu.Unlock()
				return
			}

			got, code := httpGet(key)
			if code == 404 {
				mu.Lock()
				failures++
				mu.Unlock()
				return
			}
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

func main() {
	srv := server.NewServer()
	go srv.ListenAndServe()

	waitForServer()

	testBasics()
	testConcurrentAccess()

	fmt.Println("\nAll HTTP tests completed.")
}
