package server

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"

	"distkv/store"
)

func RunNode(path1, path2 string) {
	cfg, err := LoadConfig(path1)
	if err != nil {
		fmt.Printf("fatal: %v\n", err)
		return
	}
	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		fmt.Printf("fatal: %v\n", err)
		return
	}

	defer listener.Close()

	snap := LoadSnap(path2)
	SyncWithLeader(cfg.Leader, snap.store)

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
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return
	}

	defer conn.Close()

	fmt.Fprintf(conn, "SYNC|%d\n", st.Version())

	resp, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return
	}

	if strings.HasPrefix(resp, "SYNC_DATA|") {
		if err := st.ApplySnapshot([]byte(strings.TrimPrefix(resp, "SYNC_DATA|"))); err != nil {
			return
		}
	}
}

func (snap *Snap) HandleNodeConnection(conn net.Conn) {
	defer conn.Close()

	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil && len(line) == 0 {
		return
	}
	fields := DecodeRequest(line)
	if len(fields) < 2 {
		fmt.Printf("Error!! no argument passed\n")
		return
	}

	method := fields[0]

	if method == "GET" {
		key := fields[1]

		val, ok := snap.store.Get(key)
		if !ok {
			return
		}
		fmt.Fprintf(conn, "%s", val)
	}

	if method == "PUT" {
		if len(fields) < 3 {
			fmt.Printf("Error!! not enough arguments found\n")
			return
		}
		key := fields[1]
		val := fields[2]

		err = snap.store.Set(key, val)
		if err != nil {
			fmt.Printf("fatal: %v\n", err)
			return
		} else {
			fmt.Fprintf(conn, "Key: %s succesfully set with value: %s", key, val)
		}
	}

	if method == "DELETE" {
		key := fields[1]
		err := snap.store.Delete(key)
		if err != nil {
			fmt.Printf("fatal: %v\n", err)
			return
		} else {
			fmt.Fprintf(conn, "Key: %s succesfully deleted", key)
		}
	}
}
