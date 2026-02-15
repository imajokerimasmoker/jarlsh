package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSearchNfoFiles(t *testing.T) {
	// Create a temporary directory for tests
	tmpDir, err := os.MkdirTemp("", "nfo-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Setup test files
	files := map[string]string{
		"file1.nfo":        "hello world",
		"file2.nfo":        "goodbye world",
		"subdir/file3.nfo": "hello again",
		"file4.txt":        "hello text",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err != nil {
			t.Fatalf("Failed to create subdir: %v", err)
		}
		err = os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
	}

	tests := []struct {
		query    string
		expected []string
	}{
		{"hello", []string{"file1.nfo", "subdir/file3.nfo"}},
		{"world", []string{"file1.nfo", "file2.nfo"}},
		{"goodbye", []string{"file2.nfo"}},
		{"nonexistent", []string{}},
	}

	for _, tt := range tests {
		results, err := searchNfoFiles(tmpDir, tt.query)
		if err != nil {
			t.Errorf("searchNfoFiles(%q) returned error: %v", tt.query, err)
			continue
		}

		if len(results) != len(tt.expected) {
			t.Errorf("searchNfoFiles(%q) returned %d results, want %d", tt.query, len(results), len(tt.expected))
		}

		// Check if all expected files are in results
		for _, exp := range tt.expected {
			found := false
			for _, res := range results {
				rel, _ := filepath.Rel(tmpDir, res.FilePath)
				// Normalize path for comparison on different OS if needed,
				// but here we are in a linux-like environment.
				if filepath.ToSlash(rel) == filepath.ToSlash(exp) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("searchNfoFiles(%q) did not find expected file %s", tt.query, exp)
			}
		}
	}
}
