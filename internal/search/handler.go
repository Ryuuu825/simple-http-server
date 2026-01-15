package search

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"simple.http.server/internal/config"
)

// FileInfo represents search result
type FileInfo struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	IsDir    bool   `json:"is_dir"`
	Modified string `json:"modified"`
}

// Handler manages file search
type Handler struct {
	config *config.Config
}

// NewHandler creates a new search handler
func NewHandler(cfg *config.Config) *Handler {
	return &Handler{config: cfg}
}

// ServeHTTP handles search requests
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get query parameters
	query := strings.ToLower(r.URL.Query().Get("q"))
	if query == "" {
		http.Error(w, "Query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	searchPath := r.URL.Query().Get("path")
	if searchPath == "" {
		searchPath = "/"
	}

	fileType := strings.ToLower(r.URL.Query().Get("type")) // "file", "dir", or empty for all
	maxResults := 100

	// Get base directory
	baseDir := h.config.GetFileServerDir()
	fullPath := filepath.Join(baseDir, filepath.Clean(searchPath))

	// Security check
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	absSearch, err := filepath.Abs(fullPath)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if !strings.HasPrefix(absSearch, absBase) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Search files
	results := []FileInfo{}
	err = filepath.Walk(absSearch, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		if len(results) >= maxResults {
			return filepath.SkipDir
		}

		// Get relative path
		relPath, err := filepath.Rel(absBase, path)
		if err != nil {
			return nil
		}

		// Skip the root itself
		if path == absSearch {
			return nil
		}

		// Filter by type
		if fileType == "file" && info.IsDir() {
			return nil
		}
		if fileType == "dir" && !info.IsDir() {
			return nil
		}

		// Check if name matches query
		fileName := strings.ToLower(info.Name())
		if strings.Contains(fileName, query) {
			results = append(results, FileInfo{
				Name:     info.Name(),
				Path:     "/" + filepath.ToSlash(relPath),
				Size:     info.Size(),
				IsDir:    info.IsDir(),
				Modified: info.ModTime().Format(time.RFC3339),
			})
		}

		return nil
	})

	if err != nil {
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	// Return results
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"query":   query,
		"results": results,
		"count":   len(results),
	})
}
