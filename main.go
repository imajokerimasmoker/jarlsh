package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const maxFileSize = 10 * 1024 * 1024 // 10MB limit for .nfo files

// SearchResult represents a file found during a search.
type SearchResult struct {
	FilePath string `json:"file_path"`
}

// SearchResponse is the JSON response from the search endpoint.
type SearchResponse struct {
	Query string         `json:"query"`
	Dir   string         `json:"dir"`
	Files []SearchResult `json:"files,omitempty"`
	Error string         `json:"error,omitempty"`
}

// searchNfoFiles searches for the query string recursively in .nfo files starting from root.
func searchNfoFiles(root, query string) ([]SearchResult, error) {
	var results []SearchResult
	queryBytes := []byte(query)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("Error accessing path %s: %v", path, err)
			return nil
		}
		if !d.IsDir() && strings.ToLower(filepath.Ext(path)) == ".nfo" {
			info, err := d.Info()
			if err != nil {
				return nil
			}
			if info.Size() > maxFileSize {
				log.Printf("Skipping file %s: too large (%d bytes)", path, info.Size())
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				log.Printf("Error reading file %s: %v", path, err)
				return nil
			}
			if bytes.Contains(content, queryBytes) {
				results = append(results, SearchResult{FilePath: path})
			}
		}
		return nil
	})
	return results, err
}

// searchHandler handles the /search GET request.
func searchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	dir := r.URL.Query().Get("dir")

	if query == "" || dir == "" {
		http.Error(w, "Missing 'q' or 'dir' query parameter. Example: /search?q=mysearchstring&dir=./testdir", http.StatusBadRequest)
		return
	}

	// Simple path traversal protection: don't allow absolute paths or going up if we want to restrict to a base.
	// For now, let's just make sure it stays within the current working directory if it starts with "./"
	// or allow any path if we trust the user.
	// Given the prompt "a specified Dirs", let's assume the user knows what they are doing but we'll clean it.

	cleanDir := filepath.Clean(dir)

	// Optional: restrict to current directory or a configured base directory
	// baseDir, _ := os.Getwd()
	// if !strings.HasPrefix(filepath.Join(baseDir, cleanDir), baseDir) { ... }

	log.Printf("Searching for %q in %s", query, cleanDir)

	if _, err := os.Stat(cleanDir); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("Directory %s does not exist", cleanDir), http.StatusNotFound)
		return
	}

	results, err := searchNfoFiles(cleanDir, query)

	w.Header().Set("Content-Type", "application/json")

	if err != nil {
		log.Printf("Search error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(SearchResponse{
			Query: query,
			Dir:   dir,
			Error: err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(SearchResponse{
		Query: query,
		Dir:   dir,
		Files: results,
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/search", searchHandler)

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Printf("Starting NFO Searcher server on :%s...", port)
	log.Printf("Example usage: curl \"http://localhost:%s/search?q=pattern&dir=.\" ", port)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
