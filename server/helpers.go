package server

import (
	"distkv/store"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// Config describes the cluster settings loaded from the server configuration file.
type Config struct {
	Role   string   `json:"role"`
	Addr   string   `json:"address"`
	Peers  []string `json:"peers"`
	Leader string   `json:"leader"`
}

// Snap wraps a persistent store for a node and gives the server code a common state handle.
type Snap struct {
	store *store.Store
}

type pingResult struct {
	peer    string
	isAlive bool
}

type peerRegistry struct {
	alive    map[string]bool
	reqChans map[string]chan string
	mu       sync.RWMutex
}

// LoadSnap creates the in-memory snapshot-backed store used by a node or leader.
func LoadSnap(path string) (*Snap, error) {
	st, err := store.NewStore(path)
	if err != nil {
		return nil, err
	}

	return &Snap{store: st}, err
}

// Encoder escapes special characters so request fields remain parseable across the wire protocol.
func Encoder(operand string) string {
	var sb strings.Builder
	oplength := len(operand)
	for i := 0; i < oplength; i++ {
		c := operand[i]
		switch c {
		case '|':
			sb.WriteString(`\|`)
		case '\\':
			sb.WriteString(`\\`)
		case '\n':
			sb.WriteString(`\n`)
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

// EncodeRequest serializes a request into the compact pipe-delimited format used by the server protocol.
func EncodeRequest(method, key, val string) string {
	if method != "PUT" {
		return method + "|" + Encoder(key) + "\n"
	} else {
		return method + "|" + Encoder(key) + "|" + Encoder(val) + "\n"
	}
}

// DecodeRequest reverses the wire encoding and splits a request into its logical fields.
func DecodeRequest(request string) []string {
	request = strings.TrimSuffix(request, "\n")
	var fields []string
	var cur strings.Builder
	escaped := false

	for i := 0; i < len(request); i++ {
		c := request[i]
		switch {
		case escaped:
			switch c {
			case 'n':
				cur.WriteByte('\n')
			case '|':
				cur.WriteByte('|')
			case '\\':
				cur.WriteByte('\\')
			default:
				cur.WriteByte('\\')
				cur.WriteByte(c)
			}
			escaped = false
		case c == '\\':
			escaped = true
		case c == '|':
			fields = append(fields, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if escaped {
		cur.WriteByte('\\')
	}
	return append(fields, cur.String())
}

// LoadConfig reads and unmarshals the JSON config file used to bootstrap the node.
func LoadConfig(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var conf Config

	err = json.Unmarshal(file, &conf)
	if err != nil {
		return nil, err
	}

	return &conf, err
}

// ForwardToAllPeers sends a mutation to all known healthy peers using the registry's request queue.
func ForwardToAllPeers(method, key, val string, peers []string, registry *peerRegistry) {
	req := EncodeRequest(method, key, val)

	registry.mu.RLock()
	defer registry.mu.RUnlock()

	for _, peer := range peers {

		if registry.alive[peer] {

			select {
			case registry.reqChans[peer] <- req:
			default:
				fmt.Printf("Warning: request channel for peer %s is full, skipping\n", peer)
			}
		}
	}
}

// ForwardRawToPeer delivers a pre-encoded request directly to a peer address over TCP.
func ForwardRawToPeer(peer, req string) error {

	addr := "127.0.0.1" + peer
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	_, err = conn.Write([]byte(req))
	return err
}
