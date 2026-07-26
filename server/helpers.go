package server

import (
	"distkv/store"
	"encoding/json"
	"fmt"
	"net"
	"os"
)

type Config struct {
	Role  string   `json:"role"`
	Addr  string   `json:"address"`
	Peers []string `json:"peers"`
}
type Snap struct {
	store *store.Store
}

func LoadSnap(path string) *Snap {
	return &Snap{store: store.NewStore(path)}
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
	if method != "PUT" {
		req = fmt.Sprintf("%s|%s", method, key)
	} else {
		req = fmt.Sprintf("%s|%s|%s", method, key, val)
	}
	_, err = conn.Write([]byte(req))
	return err
}

func ForwardToAllPeers(method, key, val string, peers []string) {
	for _, peer := range peers {
		go ForwardToPeer(peer, method, key, val)
	}
}
