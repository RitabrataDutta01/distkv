package store

import (
	"fmt"
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
	err := s.load()
	if err != nil {
		fmt.Println(err)
	}
	return s
}

func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[key]
	return val, ok
}

func (s *Store) Set(key, val string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = val
	return s.save()
}

func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return s.save()
}
