package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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
		name     string
		query    string
		expected []string // expected in order
	}{
		{"hello matches", "hello", []string{"subdir/file3.nfo", "file1.nfo"}}, // file3 is newer if created later? actually WalkDir order might vary, but we sort by ModTime.
		{"world matches", "world", []string{"file2.nfo", "file1.nfo"}},        // file2 is newer than file1
		{"goodbye matches", "goodbye", []string{"file2.nfo"}},
		{"none", "nonexistent", []string{}},
	}

	// Adjust creation times to ensure deterministic sorting
	// file1.nfo created first
	// file2.nfo created second
	// subdir/file3.nfo created third
	now := time.Now()
	os.Chtimes(filepath.Join(tmpDir, "file1.nfo"), now, now.Add(-10*time.Minute))
	os.Chtimes(filepath.Join(tmpDir, "file2.nfo"), now, now.Add(-5*time.Minute))
	os.Chtimes(filepath.Join(tmpDir, "subdir/file3.nfo"), now, now)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := searchNfoFiles(tmpDir, tt.query)
			if err != nil {
				t.Fatalf("searchNfoFiles(%q) returned error: %v", tt.query, err)
			}

			if len(results) != len(tt.expected) {
				t.Errorf("searchNfoFiles(%q) returned %d results, want %d", tt.query, len(results), len(tt.expected))
			}

			// Check if results are in expected order
			for i, exp := range tt.expected {
				if i >= len(results) {
					break
				}
				rel, _ := filepath.Rel(tmpDir, results[i].FilePath)
				if filepath.ToSlash(rel) != filepath.ToSlash(exp) {
					t.Errorf("searchNfoFiles(%q) result[%d] = %s, want %s", tt.query, i, rel, exp)
				}
			}
		})
	}
}

func TestValidateDir(t *testing.T) {
	wd, _ := os.Getwd()
	tmpDir, _ := os.MkdirTemp("", "validate-dir-test")
	defer os.RemoveAll(tmpDir)

	subDir := filepath.Join(tmpDir, "sub")
	os.Mkdir(subDir, 0755)

	tests := []struct {
		name        string
		allowedRoot string
		targetDir   string
		wantErr     bool
	}{
		{"within root", tmpDir, "sub", false},
		{"same as root", tmpDir, ".", false},
		{"outside root (parent)", subDir, "..", true},
		{"outside root (absolute)", subDir, wd, true},
		{"non-existent", tmpDir, "nonexistent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateDir(tt.allowedRoot, filepath.Join(tt.allowedRoot, tt.targetDir))
			if tt.name == "outside root (absolute)" {
				_, err = validateDir(tt.allowedRoot, tt.targetDir)
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("validateDir() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
