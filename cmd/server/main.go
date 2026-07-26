package main

import (
	"distkv/server"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run ./cmd/server/ <config-path>")
		return
	}
	cfgPath := os.Args[1]
	cfg, err := server.LoadConfig(cfgPath)
	if err != nil {
		fmt.Println("Failed to load config:", err)
		return
	}

	snapPath := "snapshot/" + strings.TrimPrefix(cfg.Addr, ":") + ".json"

	if cfg.Role == "primary" {
		server.RunServer(cfgPath, snapPath)
	} else {
		server.RunNode(cfgPath, snapPath)
	}
}
