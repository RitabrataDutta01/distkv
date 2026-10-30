package store

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

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
		data:    make(map[string]string),
		path:    path,
		log:     file,
		logChan: make(chan string, 10000),
	}

	err = s.load()
	if err != nil {
		fmt.Println(err)
	}

	go s.StartCompaction()

	s.wg.Add(1)
	go s.walFlusher()

	return s
}

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

func (s *Store) flushLog() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.log.Sync(); err != nil {
		fmt.Println("Error syncing log:", err)
	}
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

		freshMap := make(map[string]string, len(s.data))
		for k, v := range s.data {
			freshMap[k] = v
		}

		s.data = freshMap

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

	if s.isClosed {
		return fmt.Errorf("store is closed")
	}

	s.data[key] = val

	s.logChan <- fmt.Sprintf("SET|%s|%s\n", key, val)

	return nil
}

func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isClosed {
		return fmt.Errorf("store is closed")
	}

	delete(s.data, key)

	s.logChan <- fmt.Sprintf("DELETE|%s\n", key)

	return nil
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
