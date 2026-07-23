package server

import (
	"distkv/store"
	"fmt"
	"io"
	"net/http"
)

type Server struct {
	store *store.Store
}

func NewServer() *Server {
	return &Server{store: store.NewStore("snapshot/data.json")}
}

func (srv *Server) ListenAndServe() error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{id}", srv.handleGet)
	mux.HandleFunc("PUT /{id}", srv.handleSet)
	mux.HandleFunc("DELETE /{id}", srv.handleDelete)

	fmt.Println("Starting server at: 8080 ......")
	return http.ListenAndServe(":8080", mux)
}

func (srv *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("id")
	val, ok := srv.store.Get(key)
	if !ok {
		http.Error(w, "Key not found", http.StatusNotFound)
		return
	}
	fmt.Fprintf(w, "%s\n", val)
}

func (srv *Server) handleSet(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("id")
	val, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	srv.store.Set(key, string(val))
	fmt.Fprintf(w, "Successfully set %s for key %s\n", string(val), key)
}

func (srv *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("id")
	srv.store.Delete(key)
	fmt.Fprintf(w, "Successfully deleted key: %s\n", key)
}
