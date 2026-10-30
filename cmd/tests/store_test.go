package tests

import (
	"distkv/store"
	"path/filepath"
	"sync"
	"testing"
)

func tempSnap(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "snapshot.json")
}

func TestSetGetDelete(t *testing.T) {

	s := store.NewStore(tempSnap(t))

	if _, ok := s.Get("missing"); ok {
		t.Fatal("expected missing key to be absent")
	}

	if err := s.Set("key1", "value1"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	if v, ok := s.Get("key1"); !ok || v != "value1" {
		t.Fatalf("Got returned (%q, %v), expected ('value1', true)", v, ok)
	}

	if err := s.Delete("key1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if _, ok := s.Get("key1"); ok {
		t.Fatal("Expected key to be deleted, but it was found")
	}

}

func TestConcurrentSetGet(t *testing.T) {

	s := store.NewStore(tempSnap(t))

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)

		go func(n int) {
			defer wg.Done()

			key := "key" + string(rune('0'+n%10))

			if err := s.Set(key, "concurrent_val"); err != nil {
				t.Errorf("Concurrent Set failed: %v", err)
				return
			}

			s.Get(key)

		}(i)
	}

	wg.Wait()
}
