package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Store is the durable key/value state for a node.
// It keeps the in-memory map, appends mutation records to a WAL, and can compact snapshots on a timer.
type Store struct {
	data     map[string]string
	mu       sync.RWMutex
	path     string
	version  int
	savedAt  int64
	log      *os.File
	logChan  chan string
	wg       sync.WaitGroup
	isClosed bool
}

// Version returns the current mutation count for the store.
func (s *Store) Version() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}

// SavedAt returns when the store was last snapshotted.
func (s *Store) SavedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Unix(0, s.savedAt)
}

// NewStore initializes a store from disk and starts the WAL-flush and compaction background workers.
func NewStore(path string) (*Store, error) {

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("Failed to create WAL directory: %v", err)
	}

	file, err := os.OpenFile(path+".log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("error opening log file: %v", err)
	}

	s := &Store{
		data:    make(map[string]string),
		path:    path,
		log:     file,
		logChan: make(chan string, 10000),
	}

	err = s.load()
	if err != nil {
		return nil, fmt.Errorf("error loading store: %v", err)
	}

	go s.StartCompaction()

	s.wg.Add(1)
	go s.walFlusher()

	return s, nil
}

// walFlusher periodically flushes buffered log writes to disk so recent mutations survive crashes.
func (s *Store) walFlusher() {

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	defer s.wg.Done()

	for {
		select {
		case <-ticker.C:
			s.flushLog()
		case msg, ok := <-s.logChan:
			if !ok {
				s.flushLog()
				return
			}
			if _, err := s.log.WriteString(msg); err != nil {
				fmt.Println("Error writing to log:", err)
			}
		}
	}

}

// flushLog forces the WAL file contents to disk.
func (s *Store) flushLog() {

	if err := s.log.Sync(); err != nil {
		fmt.Println("Error syncing log:", err)
	}

}

// StartCompaction periodically snapshots the state and truncates the WAL to keep it bounded over time.
func (s *Store) StartCompaction() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {

		s.mu.Lock()
		if err := s.save(); err != nil {
			fmt.Println("Error during compaction:", err)
			continue
		}

		if err := s.log.Truncate(0); err == nil {
			s.log.Seek(0, 0)
		}

		freshMap := make(map[string]string, len(s.data))
		for k, v := range s.data {
			freshMap[k] = v
		}

		s.data = freshMap

		s.mu.Unlock()
	}

}

// Get fetches a value for the given key and reports whether it exists.
func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[key]
	return val, ok
}

// Set writes a value, increments the version, and appends the mutation to the WAL.
func (s *Store) Set(key, val string) error {
	s.mu.Lock()

	if s.isClosed {
		return fmt.Errorf("store is closed")
	}

	s.data[key] = val
	s.version++

	s.mu.Unlock()

	s.logChan <- fmt.Sprintf("SET|%s|%s\n", encodeWAL(key), encodeWAL(val))

	return nil
}

// Delete removes a key, increments the version, and persists the removal to the WAL.
func (s *Store) Delete(key string) error {
	s.mu.Lock()

	if s.isClosed {
		return fmt.Errorf("store is closed")
	}

	delete(s.data, key)
	s.version++

	s.mu.Unlock()

	s.logChan <- fmt.Sprintf("DELETE|%s\n", encodeWAL(key))

	return nil
}

// Snapshot serializes the current state as JSON for leader/follower synchronization.
func (s *Store) Snapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(fileFormat{
		Version:   s.version,
		Timestamp: s.savedAt,
		Data:      s.data,
	})
}

// ApplySnapshot replaces the in-memory state with a remote snapshot and writes it to disk.
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

// Close shuts down the WAL worker and closes the log file cleanly.
func (s *Store) Close() error {

	s.mu.Lock()
	if s.isClosed {
		s.mu.Unlock()
		return nil
	}
	s.isClosed = true
	s.mu.Unlock()

	close(s.logChan)

	s.wg.Wait()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.log != nil {
		return s.log.Close()
	}
	return nil
}

// encodeWAL escapes pipe and newline characters so they can be safely stored in the WAL format.
func encodeWAL(operand string) string {
	var sb strings.Builder

	for i := 0; i < len(operand); i++ {
		c := operand[i]
		switch c {
		case '\n':
			sb.WriteString("\\n")
		case '|':
			sb.WriteString("\\|")
		case '\\':
			sb.WriteString("\\\\")
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String()
}
