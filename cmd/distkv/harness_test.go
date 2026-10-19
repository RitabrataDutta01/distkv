package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"distkv/server"
)

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found in any parent directory")
		}
		dir = parent
	}
}

func TestHarnessScenarios(t *testing.T) {
	if err := os.Chdir(findRepoRoot(t)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("snapshot", 0755); err != nil {
		t.Fatal(err)
	}

	go server.RunServer("server/config-test.json", "snapshot/data.json")
	waitForServer()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w

	testBasics()
	testEscaping()
	testConcurrentAccess()
	testSaveFailure()
	testCorruptedSnapshot()

	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)

	if strings.Contains(string(out), "FAIL:") {
		t.Errorf("harness reported failures:\n%s", out)
	}
}
