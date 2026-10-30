package tests

import (
	"distkv/server"
	"testing"
)

func TestDecodeRequest(t *testing.T) {

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"Standard GET", "GET|/color\n", []string{"GET", "/color"}},
		{"Standard PUT", "PUT|k|v\n", []string{"PUT", "k", "v"}},
		{"Missing Value", "GET\n", []string{"GET"}},
		{"Escaped Pipe", `PUT|k\|1|v` + "\n", []string{"PUT", "k|1", "v"}},
		{"Trailing Slash", "a\\", []string{"a\\"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := server.DecodeRequest(tc.input)
			if len(got) != len(tc.expected) {
				t.Fatalf("Expected %d parts, got %d", len(tc.expected), len(got))
			}
			for i := range got {
				if got[i] != tc.expected[i] {
					t.Errorf("Mismatch at index %d: expected %q, got %q", i, tc.expected[i], got[i])
				}
			}
		})
	}
}
