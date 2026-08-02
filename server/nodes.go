package server

import (
	"fmt"
	"net"
	"strings"
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

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("fatal: %v\n", err)
			return
		}

		go snap.HandleNodeConnection(conn)
	}

}

func (snap *Snap) HandleNodeConnection(conn net.Conn) {
	defer conn.Close()

	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	if err != nil {
		fmt.Printf("fatal: %v\n", err)
		return
	}
	rawText := string(buffer[:n])
	parts := strings.Split(rawText, "|")

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
		key := parts[1]
		val := parts[2]

		err = snap.store.Set(key, val)
		if err != nil {
			fmt.Printf("fatal: %v\n", err)
			return
		} else {
			fmt.Fprintf(conn, "Key: %s succesfully set with value: %s", key, val)
		}
	}

	if method == "DELETE" {
		key := parts[1]
		err := snap.store.Delete(key)
		if err != nil {
			fmt.Printf("fatal: %v\n", err)
			return
		} else {
			fmt.Fprintf(conn, "Key: %s succesfully deleted", key)
		}
	}
}
