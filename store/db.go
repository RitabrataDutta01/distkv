package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type fileFormat struct {
	Version   int               `json:"version"`
	Timestamp int64             `json:"timestamp"`
	Data      map[string]string `json:"data"`
}

func (s *Store) save() error {

	version := s.version + 1
	ts := time.Now().UnixNano()
	data, err := json.Marshal(fileFormat{
		Version:   version,
		Timestamp: ts,
		Data:      s.data,
	})

	if err != nil {
		return err
	}

	if err := s.writeFile(data); err != nil {
		return err
	}

	s.version = version
	s.savedAt = ts
	return nil

}

func (s *Store) writeFile(data []byte) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	tmpPath := s.path + ".tmp"

	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	defer os.Remove(tmpPath)

	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}

	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		return err
	}
	return nil

}

func (s *Store) load() error {
	file, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}

	var ff fileFormat

	if err := json.Unmarshal(file, &ff); err != nil {
		return err
	}

	if ff.Data == nil {
		var legacy map[string]string
		if err := json.Unmarshal(file, &legacy); err != nil {
			return err
		}
		ff.Data = legacy
	}

	s.data = ff.Data
	s.version = ff.Version
	s.savedAt = ff.Timestamp

	return nil
}
