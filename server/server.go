package server

import (
	"fmt"
	"net"
	"strconv"
	"strings"
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

	snap := LoadSnap(path2)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Warning: Connection error: %v\n", err)
			continue
		}

		go snap.HandleLeaderConnection(conn, cfg.Peers)
	}
}

func (snap *Snap) HandleLeaderConnection(conn net.Conn, peers []string) {
	defer conn.Close()

	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	if err != nil {
		return
	}
	rawText := string(buffer[:n])
	parts := strings.Split(rawText, "|")

	if len(parts) < 2 {
		fmt.Fprintf(conn, "ERROR: Invalid request format\n")
		return
	}

	method := parts[0]

	if method == "GET" {
		key := parts[1]

		val, ok := snap.store.Get(key)
		if !ok {
			fmt.Fprintf(conn, "404 Not Found")
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
			failedPeers := ForwardToAllPeers("PUT", key, val, peers)
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
			failedPeers := ForwardToAllPeers("DELETE", key, "", peers)
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
