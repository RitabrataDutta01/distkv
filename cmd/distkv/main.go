package main

import (
	"distkv/store"
	"fmt"
	"sync"
)

func check(ok bool, msg string) {
	if ok {
		fmt.Println("PASS:", msg)
	} else {
		fmt.Println("FAIL:", msg)
	}
}

func testBasics(s *store.Store) {
	fmt.Println("\n=== Basic Tests ===")

	s.Set("color", "blue")
	val, ok := s.Get("color")
	check(val == "blue", "Set+Get returns correct value")
	check(ok == true, "Set+Get returns ok=true")

	val, ok = s.Get("nonexistent")
	check(val == "", "Get missing key returns zero value")
	check(ok == false, "Get missing key returns ok=false")

	s.Set("color", "red")
	val, ok = s.Get("color")
	check(val == "red", "Overwrite updates value")
	check(ok == true, "Overwrite still returns ok=true")

	s.Delete("color")
	val, ok = s.Get("color")
	check(val == "", "Delete removes key (zero value)")
	check(ok == false, "Delete removes key (ok=false)")

	s.Delete("still_nonexistent")
	check(true, "Delete non-existent key does not panic")
}

func testConcurrentAccess(s *store.Store) {
	fmt.Println("\n=== Concurrent Stress Test ===")

	var wg sync.WaitGroup
	var mu sync.Mutex
	failures := 0

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", n%10)
			val := fmt.Sprintf("val%d", n)
			s.Set(key, val)
			got, ok := s.Get(key)
			if !ok {
				mu.Lock()
				failures++
				mu.Unlock()
				return
			}
			if got == "" {
				mu.Lock()
				failures++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	check(failures == 0, fmt.Sprintf("Concurrent stress test (0 failures, got %d)", failures))
}

func main() {
	s := store.NewStore()
	testBasics(s)
	testConcurrentAccess(s)
	fmt.Println("\nAll tests completed.")
}
