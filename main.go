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
	"sort"
	"strings"
	"time"
)

const maxFileSize = 10 * 1024 * 1024 // 10MB limit for .nfo files

var allowedRoot string

// SearchResult represents a file found during a search.
type SearchResult struct {
	FilePath string    `json:"file_path"`
	Link     string    `json:"link"`
	ModTime  time.Time `json:"mod_time"`
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
				rel, err := filepath.Rel(root, path)
				link := ""
				if err == nil {
					dir := filepath.Dir(rel)
					if dir == "." {
						link = "/"
					} else {
						link = "/" + filepath.ToSlash(dir)
					}
				}
				results = append(results, SearchResult{
					FilePath: path,
					Link:     link,
					ModTime:  info.ModTime(),
				})
			}
		}
		return nil
	})

	if err == nil {
		// Sort results by modification time (newest first)
		sort.Slice(results, func(i, j int) bool {
			return results[i].ModTime.After(results[j].ModTime)
		})
	}

	return results, err
}

// validateDir checks if the target directory is within the allowed root.
func validateDir(allowedRoot, targetDir string) (string, error) {
	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return "", fmt.Errorf("invalid directory path")
	}

	rel, err := filepath.Rel(allowedRoot, absTarget)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("access denied: search is restricted to subdirectories of %s", allowedRoot)
	}

	if _, err := os.Stat(absTarget); os.IsNotExist(err) {
		return "", fmt.Errorf("directory %s does not exist", targetDir)
	}

	return absTarget, nil
}

// searchHandler handles the /search GET request.
func searchHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Incoming request: %s %s", r.Method, r.URL.Path)
	query := r.URL.Query().Get("q")
	dir := r.URL.Query().Get("dir")

	if query == "" || dir == "" {
		http.Error(w, "Missing 'q' or 'dir' query parameter. Example: /search?q=mysearchstring&dir=./testdir", http.StatusBadRequest)
		return
	}

	cleanDir, err := validateDir(allowedRoot, dir)
	if err != nil {
		if strings.Contains(err.Error(), "access denied") {
			http.Error(w, err.Error(), http.StatusForbidden)
		} else if strings.Contains(err.Error(), "does not exist") {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}

	log.Printf("Searching for %q in %s", query, cleanDir)

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

	allowedRoot = os.Getenv("ALLOWED_ROOT")
	if allowedRoot == "" {
		var err error
		allowedRoot, err = os.Getwd()
		if err != nil {
			log.Fatalf("Error getting working directory: %v", err)
		}
	} else {
		var err error
		allowedRoot, err = filepath.Abs(allowedRoot)
		if err != nil {
			log.Fatalf("Error resolving allowed root: %v", err)
		}
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/search/", searchHandler)

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Serve Angular frontend (optional)
	if os.Getenv("SERVE_FRONTEND") == "true" {
		distPath := "./frontend/dist/frontend/browser"

		// Check if dist exists, if not maybe it is just dist/frontend
		if _, err := os.Stat(distPath); os.IsNotExist(err) {
			distPath = "./frontend/dist/frontend"
		}

		if _, err := os.Stat(distPath); err == nil {
			log.Printf("Serving frontend from %s", distPath)
			fileServer := http.FileServer(http.Dir(distPath))

			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				// If the request is for a file that doesn't exist, serve index.html (SPA routing)
				path := filepath.Join(distPath, r.URL.Path)
				_, err := os.Stat(path)
				if os.IsNotExist(err) {
					http.ServeFile(w, r, filepath.Join(distPath, "index.html"))
					return
				}
				fileServer.ServeHTTP(w, r)
			})
		} else {
			log.Printf("Frontend directory %s not found, skipping frontend serving", distPath)
		}
	} else {
		log.Printf("Frontend serving is disabled (SERVE_FRONTEND != true)")
	}

	log.Printf("Starting NFO Searcher server on :%s...", port)
	log.Printf("API example: curl \"http://localhost:%s/search/?q=pattern&dir=.\" ", port)

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
