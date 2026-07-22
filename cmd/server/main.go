package main

import (
	"distkv/server"
	"log"
)

func main() {
	srv := server.NewServer()
	log.Fatal(srv.ListenAndServe())
}
