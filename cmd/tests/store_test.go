package tests

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"distkv/store"
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

	if err := s.Set("k", "v"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if v, ok := s.Get("k"); !ok || v != "v" {
		t.Fatalf("Get = (%q,%v), want (v,true)", v, ok)
	}

	if err := s.Set("k", "v2"); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	if v, _ := s.Get("k"); v != "v2" {
		t.Fatalf("after overwrite Get = %q, want v2", v)
	}

	if err := s.Delete("k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := s.Get("k"); ok {
		t.Fatal("expected key deleted")
	}

	if err := s.Delete("never-existed"); err != nil {
		t.Fatalf("Delete missing should not error: %v", err)
	}
}

func TestVersionIncrements(t *testing.T) {
	s := store.NewStore(tempSnap(t))
	if s.Version() != 0 {
		t.Fatalf("fresh version = %d, want 0", s.Version())
	}
	if err := s.Set("a", "1"); err != nil {
		t.Fatal(err)
	}
	if s.Version() != 1 {
		t.Fatalf("version after Set = %d, want 1", s.Version())
	}
	if err := s.Set("a", "2"); err != nil {
		t.Fatal(err)
	}
	if s.Version() != 2 {
		t.Fatalf("version after 2nd Set = %d, want 2", s.Version())
	}
	if err := s.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if s.Version() != 3 {
		t.Fatalf("version after Delete = %d, want 3", s.Version())
	}
}

func TestPersistenceRoundTrip(t *testing.T) {
	path := tempSnap(t)
	s := store.NewStore(path)
	if err := s.Set("color", "blue"); err != nil {
		t.Fatal(err)
	}

	s2 := store.NewStore(path)
	if v, ok := s2.Get("color"); !ok || v != "blue" {
		t.Fatalf("reload Get = (%q,%v), want (blue,true)", v, ok)
	}
	if s2.Version() != s.Version() {
		t.Fatalf("reload version = %d, want %d", s2.Version(), s.Version())
	}
}

func TestSnapshotApplyRoundTrip(t *testing.T) {
	s := store.NewStore(tempSnap(t))
	s.Set("a", "1")
	s.Set("b", "2")

	raw, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	dst := store.NewStore(tempSnap(t))
	if err := dst.ApplySnapshot(raw); err != nil {
		t.Fatalf("ApplySnapshot: %v", err)
	}

	for _, k := range []string{"a", "b"} {
		want, _ := s.Get(k)
		if got, ok := dst.Get(k); !ok || got != want {
			t.Fatalf("applied Get(%q) = (%q,%v), want (%q,true)", k, got, ok, want)
		}
	}
	if dst.Version() != s.Version() {
		t.Fatalf("applied version = %d, want %d", dst.Version(), s.Version())
	}
	if !dst.SavedAt().Equal(s.SavedAt()) {
		t.Fatalf("applied SavedAt = %v, want %v", dst.SavedAt(), s.SavedAt())
	}
}

func TestLegacySnapshotLoad(t *testing.T) {
	path := tempSnap(t)
	if err := os.WriteFile(path, []byte(`{"legacy":"value"}`), 0644); err != nil {
		t.Fatal(err)
	}
	s := store.NewStore(path)
	if v, ok := s.Get("legacy"); !ok || v != "value" {
		t.Fatalf("legacy Get = (%q,%v), want (value,true)", v, ok)
	}
	if s.Version() != 0 {
		t.Fatalf("legacy version = %d, want 0", s.Version())
	}
}

func TestCorruptSnapshotStartsEmpty(t *testing.T) {
	path := tempSnap(t)
	if err := os.WriteFile(path, []byte("{{{not json}}}"), 0644); err != nil {
		t.Fatal(err)
	}
	s := store.NewStore(path)
	if _, ok := s.Get("anything"); ok {
		t.Fatal("expected empty store after corrupt snapshot")
	}
}

func TestSaveLeavesNoTempFile(t *testing.T) {
	path := tempSnap(t)
	s := store.NewStore(path)
	if err := s.Set("k", "v"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("expected no .tmp file, stat err = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected snapshot file to exist: %v", err)
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := store.NewStore(tempSnap(t))
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "key" + string(rune('0'+n%10))
			if err := s.Set(key, "v"); err != nil {
				return
			}
			s.Get(key)
		}(i)
	}
	wg.Wait()
}
