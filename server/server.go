package server

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

func RunServer(path1, path2 string) {
	cfg, err := LoadConfig(path1)
	if err != nil {
		fmt.Printf("Fatal: failed to load config: %v\n", err)
		return
	}

	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		fmt.Printf("Fatal: %v\n", err)
		return
	}

	defer listener.Close()

	registry := &peerRegistry{
		alive:    make(map[string]bool),
		reqChans: make(map[string]chan string),
	}

	for _, peer := range cfg.Peers {
		registry.alive[peer] = true
		registry.reqChans[peer] = make(chan string, 1000)

		go func(p string, ch chan string) {
			for req := range ch {

				err := ForwardRawToPeer(p, req)
				if err != nil {
					registry.mu.Lock()
					registry.alive[p] = false
					registry.mu.Unlock()
				}
			}
		}(peer, registry.reqChans[peer])
	}

	go healthCheckEngine(cfg.Peers, registry)

	snap, err := LoadSnap(path2)
	if err != nil {
		fmt.Printf("Fatal: failed to load snapshot/WAL: %v\n", err)
		return
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Warning: Connection error: %v\n", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		go snap.HandleLeaderConnection(conn, cfg.Peers, registry)
	}
}

func healthCheckEngine(peers []string, registry *peerRegistry) {

	healthy, dead := healthCheck(peers)
	writeReg(registry, healthy, dead)

	ticker := time.NewTicker(1 * time.Minute)

	for range ticker.C {
		healthy, dead := healthCheck(peers)
		writeReg(registry, healthy, dead)
	}
}

func writeReg(registry *peerRegistry, healthy []string, dead []string) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	for _, h := range healthy {
		registry.alive[h] = true
	}
	for _, d := range dead {
		registry.alive[d] = false
	}
}

func healthCheck(peers []string) ([]string, []string) {
	var healthy, dead []string
	resultChan := make(chan pingResult, len(peers))

	for _, peer := range peers {
		go func(p string) {
			addr := "127.0.0.1" + p
			timeout := 5 * time.Second
			conn, err := net.DialTimeout("tcp", addr, timeout)
			if err != nil {
				resultChan <- pingResult{peer: p, isAlive: false}
				return
			}
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(5 * time.Second))
			var req string
			req = EncodeRequest("PING", "", "")
			if _, err := conn.Write([]byte(req)); err != nil {
				resultChan <- pingResult{peer: p, isAlive: false}
				return
			}

			resp, err := bufio.NewReader(conn).ReadString('\n')
			if err != nil || strings.TrimSpace(resp) != "PONG" {
				resultChan <- pingResult{peer: p, isAlive: false}
				return
			}
			resultChan <- pingResult{peer: p, isAlive: true}
		}(peer)
	}
	for i := 0; i < len(peers); i++ {
		result := <-resultChan
		if !result.isAlive {
			dead = append(dead, result.peer)
		} else {
			healthy = append(healthy, result.peer)
		}
	}
	return healthy, dead
}

func (snap *Snap) HandleLeaderConnection(conn net.Conn, peers []string, registy *peerRegistry) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	connLimit := io.LimitedReader{R: conn, N: 1 << 20}
	reader := bufio.NewReader(&connLimit)

	line, err := reader.ReadString('\n')
	if err != nil || len(line) == 0 {
		if errors.Is(err, io.EOF) {
			if connLimit.N == 0 {
				fmt.Fprintf(conn, "Payload too big, max payload size is 1MB\n")
				return
			}
			if len(line) == 0 {
				fmt.Fprintf(conn, "No data received\n")
				return
			}
		} else {
			fmt.Fprintf(conn, "ERROR: failed to read request: %v\n", err)
		}
		return
	}
	parts := DecodeRequest(line)
	if len(parts) < 2 {
		fmt.Fprintf(conn, "ERROR: Invalid request format\n")
		return
	}

	method := parts[0]

	if method == "GET" {
		key := parts[1]

		val, ok := snap.store.Get(key)
		if !ok {
			fmt.Fprintf(conn, "ERROR: Key not found\n")
			return
		}
		fmt.Fprintf(conn, "%s", val)
	}

	if method == "PUT" {
		if len(parts) < 3 {
			fmt.Fprintf(conn, "ERROR: PUT request missing value\n")
			return
		}

		key := parts[1]
		val := parts[2]

		err = snap.store.Set(key, val)
		if err != nil {
			fmt.Fprintf(conn, "ERROR: Store rejected PUT: %v\n", err)
			return
		} else {
			fmt.Fprintf(conn, "Key: %s succesfully set with value: %s", key, val)
		}
		ForwardToAllPeers("PUT", key, val, peers, registy)
		return
	}

	if method == "DELETE" {
		key := parts[1]
		err := snap.store.Delete(key)
		if err != nil {
			fmt.Fprintf(conn, "ERROR: Store rejected DELETE: %v\n", err)
			return
		} else {
			fmt.Fprintf(conn, "Key: %s succesfully deleted", key)
		}
		ForwardToAllPeers("DELETE", key, "", peers, registy)
		return
	}

	if method == "SYNC" {
		if len(parts) < 2 {
			fmt.Fprintf(conn, "ERROR: SYNC request missing version\n")
			return
		}
		nodeVersion, err := strconv.Atoi(parts[1])
		if err != nil {
			fmt.Fprintf(conn, "ERROR: invalid SYNC version\n")
			return
		}
		if nodeVersion == snap.store.Version() {
			fmt.Fprintf(conn, "SYNC_OK\n")
		} else {
			snapJSON, err := snap.store.Snapshot()
			if err != nil {
				return
			}
			fmt.Fprintf(conn, "SYNC_DATA|%s\n", snapJSON)
		}
		return
	}
	fmt.Fprintf(conn, "ERROR: Unknown method: %s\n", method)

}
