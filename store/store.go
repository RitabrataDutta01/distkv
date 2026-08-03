package store

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type Store struct {
	data    map[string]string
	mu      sync.RWMutex
	path    string
	version int
	savedAt int64
}

func (s *Store) Version() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}

func (s *Store) SavedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Unix(0, s.savedAt)
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

func (s *Store) Snapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(fileFormat{
		Version:   s.version,
		Timestamp: s.savedAt,
		Data:      s.data,
	})
}

func (s *Store) ApplySnapshot(raw []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ff fileFormat

	if err := json.Unmarshal(raw, &ff); err != nil {
		return err
	}
	s.data = ff.Data
	s.version = ff.Version
	s.savedAt = ff.Timestamp

	return s.writeFile(raw)
}
