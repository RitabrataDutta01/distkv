package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}

	data, err := json.Marshal(s.data)

	if err != nil {
		return err
	}

	tmpPath := s.path + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, s.path)
}

func (s *Store) load() error {
	file, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}

	err = json.Unmarshal(file, &s.data)
	if err != nil {
		return err
	}

	return nil
}
