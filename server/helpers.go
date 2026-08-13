package server

import (
	"distkv/store"
	"encoding/json"
	"net"
	"os"
	"strings"
)

type Config struct {
	Role   string   `json:"role"`
	Addr   string   `json:"address"`
	Peers  []string `json:"peers"`
	Leader string   `json:"leader"`
}
type Snap struct {
	store *store.Store
}

func LoadSnap(path string) *Snap {
	return &Snap{store: store.NewStore(path)}
}

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

func EncodeRequest(method, key, val string) string {
	if method != "PUT" {
		return method + "|" + Encoder(key) + "\n"
	} else {
		return method + "|" + Encoder(key) + "|" + Encoder(val) + "\n"
	}
}

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

func ForwardToPeer(peer, method, key, val string) error {
	addr := "127.0.0.1" + peer
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}

	defer conn.Close()

	var req string
	req = EncodeRequest(method, key, val)
	_, err = conn.Write([]byte(req))
	return err
}

func ForwardToAllPeers(method, key, val string, peers []string) []string {
	var failedPeers []string
	resultChan := make(chan string, len(peers))
	for _, peer := range peers {
		go func(p string) {
			err := ForwardToPeer(p, method, key, val)

			if err != nil {
				resultChan <- p
			} else {
				resultChan <- ""
			}
		}(peer)
	}
	for i := 0; i < len(peers); i++ {
		failedPeer := <-resultChan
		if failedPeer != "" {
			failedPeers = append(failedPeers, failedPeer)
		}
	}
	return failedPeers
}
