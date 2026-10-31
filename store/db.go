package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type fileFormat struct {
	Version   int               `json:"version"`
	Timestamp int64             `json:"timestamp"`
	Data      map[string]string `json:"data"`
}

func (s *Store) save() error {

	ts := time.Now().UnixNano()
	data, err := json.Marshal(fileFormat{
		Version:   s.version,
		Timestamp: ts,
		Data:      s.data,
	})

	if err != nil {
		return err
	}

	if err := s.writeFile(data); err != nil {
		return err
	}

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
	if err == nil {
		var ff fileFormat
		if err := json.Unmarshal(file, &ff); err != nil {
			return fmt.Errorf("corrupt snapshot file: %v", err)
		} else {
			if ff.Data == nil {
				var legacy map[string]string
				json.Unmarshal(file, &legacy)
				ff.Data = legacy
			}
			s.data = ff.Data
			s.version = ff.Version
			s.savedAt = ff.Timestamp
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read snapshot: %v", err)
	}

	if s.data == nil {
		s.data = make(map[string]string)
	}

	lf, err := os.Open(s.path + ".log")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to open WAL: %v", err)
	}
	defer lf.Close()

	scanner := bufio.NewScanner(lf)

	for scanner.Scan() {
		parts := decodeWAL(scanner.Text())

		if len(parts) == 0 {
			continue
		}

		if parts[0] == "SET" && len(parts) == 3 {
			s.data[parts[1]] = parts[2]
		} else if parts[0] == "DELETE" && len(parts) == 2 {
			delete(s.data, parts[1])
		}
	}

	return scanner.Err()
}

func decodeWAL(request string) []string {

	request = strings.TrimSuffix(request, "\n")
	var fields []string
	var cur strings.Builder
	escaped := false

	for i := 0; i < len(request); i++ {
		c := request[i]
		switch {
		case escaped:
			switch c {
			case 'n':
				cur.WriteByte('\n')
			case '|':
				cur.WriteByte('|')
			case '\\':
				cur.WriteByte('\\')
			default:
				cur.WriteByte('\\')
				cur.WriteByte(c)
			}
			escaped = false
		case c == '\\':
			escaped = true
		case c == '|':
			fields = append(fields, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if escaped {
		cur.WriteByte('\\')
	}
	return append(fields, cur.String())
}
