package store

import (
	"sync"
)

type Store struct {
	data map[string]string
	mu   sync.RWMutex
	path string
}

func NewStore(path string) *Store {
	s := &Store{
		data: make(map[string]string),
		path: path,
	}
	s.load()
	return s
}

func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[key]
	return val, ok
}

func (s *Store) Set(key, val string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = val
	s.save()
}

func (s *Store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	s.save()
}
