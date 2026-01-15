package upload

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"simple.http.server/internal/config"
)

const (
	maxUploadSize = 500 << 20 // 500 MB
)

// Handler manages file uploads
type Handler struct {
	config *config.Config
}

// NewHandler creates a new upload handler
func NewHandler(cfg *config.Config) *Handler {
	return &Handler{config: cfg}
}

// ServeHTTP handles file upload requests
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form with size limit
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	// Get upload path (relative to server root)
	uploadPath := r.FormValue("path")
	if uploadPath == "" {
		uploadPath = "/"
	}

	// Get base directory and construct full upload path
	baseDir := h.config.GetFileServerDir()
	fullPath := filepath.Join(baseDir, filepath.Clean(uploadPath))

	// Security: verify path is within allowed directory
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	absUpload, err := filepath.Abs(fullPath)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if !strings.HasPrefix(absUpload, absBase) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Ensure upload directory exists
	if err := os.MkdirAll(absUpload, 0755); err != nil {
		http.Error(w, "Failed to create upload directory", http.StatusInternalServerError)
		return
	}

	// Process uploaded files
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "No files uploaded", http.StatusBadRequest)
		return
	}

	uploadedFiles := []string{}
	var uploadErrors []string

	for _, fileHeader := range files {
		// Open uploaded file
		file, err := fileHeader.Open()
		if err != nil {
			uploadErrors = append(uploadErrors, fmt.Sprintf("%s: failed to open", fileHeader.Filename))
			continue
		}
		defer file.Close()

		// Security: sanitize filename
		filename := filepath.Base(filepath.Clean(fileHeader.Filename))
		if filename == "." || filename == ".." {
			uploadErrors = append(uploadErrors, fmt.Sprintf("%s: invalid filename", fileHeader.Filename))
			continue
		}

		// Create destination file
		destPath := filepath.Join(absUpload, filename)
		
		// Check if file already exists and handle it
		if _, err := os.Stat(destPath); err == nil {
			// File exists, append timestamp to make it unique
			ext := filepath.Ext(filename)
			base := strings.TrimSuffix(filename, ext)
			filename = fmt.Sprintf("%s_%d%s", base, fileHeader.Size, ext)
			destPath = filepath.Join(absUpload, filename)
		}

		dst, err := os.Create(destPath)
		if err != nil {
			uploadErrors = append(uploadErrors, fmt.Sprintf("%s: failed to create file", filename))
			continue
		}

		// Copy file content
		written, err := io.Copy(dst, file)
		dst.Close()

		if err != nil {
			os.Remove(destPath) // Clean up partial file
			uploadErrors = append(uploadErrors, fmt.Sprintf("%s: failed to save", filename))
			continue
		}

		log.Printf("Uploaded: %s (%d bytes) to %s", filename, written, absUpload)
		uploadedFiles = append(uploadedFiles, filename)
	}

	// Prepare response
	response := map[string]interface{}{
		"uploaded": uploadedFiles,
		"count":    len(uploadedFiles),
	}

	if len(uploadErrors) > 0 {
		response["errors"] = uploadErrors
	}

	w.Header().Set("Content-Type", "application/json")
	if len(uploadedFiles) > 0 {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
	json.NewEncoder(w).Encode(response)
}
