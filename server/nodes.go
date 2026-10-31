package server

import (
	"bufio"
	"distkv/store"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

func RunNode(path1, path2 string) {
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

	snap, err := LoadSnap(path2)
	if err != nil {
		fmt.Printf("Fatal: failed to load snapshot/WAL: %v\n", err)
		return
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		SyncWithLeader(cfg.Leader, snap.store)
		for range ticker.C {
			SyncWithLeader(cfg.Leader, snap.store)
		}
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Warning: Connection error: %v\n", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		go snap.HandleNodeConnection(conn)
	}
}

func SyncWithLeader(leader string, st *store.Store) {
	addr := "127.0.0.1" + leader
	timeout := 5 * time.Second
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			fmt.Println("Connection timeout:", err)
		} else {
			fmt.Println("Connection error:", err)
		}
		return
	}

	defer conn.Close()

	conn.SetDeadline(time.Now().Add(5 * time.Second))

	fmt.Fprintf(conn, "SYNC|%d\n", st.Version())
	resp, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return
	}

	if strings.HasPrefix(resp, "SYNC_DATA|") {
		if err := st.ApplySnapshot([]byte(strings.TrimPrefix(resp, "SYNC_DATA|"))); err != nil {
			fmt.Printf("Error applying snapshot: %v", err)
			return
		}
		fmt.Println("Snapshot applied successfully from leader.")
	}
}

func (snap *Snap) HandleNodeConnection(conn net.Conn) {
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

	fields := DecodeRequest(line)
	if len(fields) < 2 && fields[0] != "PING" {
		fmt.Printf("Error!! no argument passed\n")
		return
	}

	method := fields[0]

	if method == "GET" {
		key := fields[1]

		val, ok := snap.store.Get(key)
		if !ok {
			fmt.Fprintf(conn, "ERROR: Key not found\n")
			return
		}
		fmt.Fprintf(conn, "%s", val)
		return
	}

	if method == "PUT" {
		if len(fields) < 3 {
			fmt.Fprintf(conn, "ERROR: PUT request missing value\n")
			return
		}
		key := fields[1]
		val := fields[2]

		err = snap.store.Set(key, val)
		if err != nil {
			fmt.Fprintf(conn, "ERROR: Store rejected PUT: %v\n", err)
			return
		} else {
			fmt.Fprintf(conn, "Key: %s succesfully set with value: %s", key, val)
		}
		return
	}

	if method == "DELETE" {
		key := fields[1]
		err := snap.store.Delete(key)
		if err != nil {
			fmt.Fprintf(conn, "ERROR: Store rejected DELETE: %v\n", err)
			return
		} else {
			fmt.Fprintf(conn, "Key: %s succesfully deleted", key)
		}
		return
	}

	if method == "PING" {
		fmt.Fprintf(conn, "PONG\n")
	}

	fmt.Fprintf(conn, "ERROR: Unknown method: %s\n", method)
}
