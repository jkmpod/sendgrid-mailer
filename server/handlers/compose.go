package handlers

import (
	"net/http"
	"sync"
)

// Package-level state for the last uploaded CSV metadata.
// Protected by a mutex because handlers run concurrently.
var (
	mu           sync.RWMutex
	lastColumns  []string
	lastFilePath string
)

// SetLastColumns stores a copy of the column names from the most recent CSV
// upload. Copying on write ensures the caller's slice and the internal store
// never share a backing array.
func SetLastColumns(cols []string) {
	mu.Lock()
	defer mu.Unlock()
	lastColumns = append([]string(nil), cols...)
}

// SetLastFilePath stores the file path from the most recent CSV upload.
func SetLastFilePath(path string) {
	mu.Lock()
	defer mu.Unlock()
	lastFilePath = path
}

// HandleCompose returns the column names and file path from the most recent
// CSV upload. This is a helper endpoint for the template editor — no
// persistence is needed.
func HandleCompose(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	cols := append([]string(nil), lastColumns...)
	path := lastFilePath
	mu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"columns":  cols,
		"filePath": path,
	})
}
