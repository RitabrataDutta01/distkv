package server

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

func RunServer(path1, path2 string) {
	cfg, err := LoadConfig(path1)
	if err != nil {
		return
	}

	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return
	}

	defer listener.Close()

	registry := &peerRegistry{
		alive: make(map[string]bool),
	}

	go healthCheckEngine(cfg.Peers, registry)

	snap := LoadSnap(path2)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Warning: Connection error: %v\n", err)
			continue
		}

		go snap.HandleLeaderConnection(conn, cfg.Peers, registry)
	}
}

func healthCheckEngine(peers []string, registry *peerRegistry) {

	ticker := time.NewTicker(5 * time.Minute)

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
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				resultChan <- pingResult{peer: p, isAlive: false}
				return
			}
			defer conn.Close()
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

	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil && len(line) == 0 {
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
			return
		} else {
			fmt.Fprintf(conn, "Key: %s succesfully set with value: %s", key, val)
		}
		go func() {
			failedPeers := ForwardToAllPeers("PUT", key, val, peers, registy)
			if len(failedPeers) > 0 {
				fmt.Printf("Warning: Failed to replicate to peers: %v\n", failedPeers)
			}
		}()
	}

	if method == "DELETE" {
		key := parts[1]
		err := snap.store.Delete(key)
		if err != nil {
			return
		} else {
			fmt.Fprintf(conn, "Key: %s succesfully deleted", key)
		}
		go func() {
			failedPeers := ForwardToAllPeers("DELETE", key, "", peers, registy)
			if len(failedPeers) > 0 {
				fmt.Printf("Warning: Failed to replicate to peers: %v\n", failedPeers)
			}
		}()
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

}
