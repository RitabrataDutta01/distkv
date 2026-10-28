package store

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type Store struct {
	data    map[string]string
	mu      sync.RWMutex
	path    string
	version int
	savedAt int64
	log     *os.File
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

	file, err := os.OpenFile(path+".log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening log file:", err)
	}

	s := &Store{
		data: make(map[string]string),
		path: path,
		log:  file,
	}

	err = s.load()
	if err != nil {
		fmt.Println(err)
	}

	go s.StartCompaction()

	return s
}

func (s *Store) StartCompaction() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {

		s.mu.Lock()
		s.save()

		if err := s.log.Truncate(0); err == nil {
			s.log.Seek(0, 0)
		}

		s.mu.Unlock()
	}

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
	_, err := fmt.Fprintf(s.log, "SET|%s|%s\n", key, val)

	if err != nil {
		return err
	}

	return s.log.Sync()
}

func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	_, err := fmt.Fprintf(s.log, "DELETE|%s\n", key)

	if err != nil {
		return err
	}

	return s.log.Sync()
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
