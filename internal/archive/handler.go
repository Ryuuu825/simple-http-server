package archive

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"simple.http.server/internal/config"
)

// Handler manages archive creation
type Handler struct {
	config *config.Config
}

// NewHandler creates a new archive handler
func NewHandler(cfg *config.Config) *Handler {
	return &Handler{config: cfg}
}

// ServeHTTP handles archive requests
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

	// Get path to archive
	archivePath := r.URL.Query().Get("path")
	if archivePath == "" {
		archivePath = "/"
	}

	// Get base directory
	baseDir := h.config.GetFileServerDir()
	fullPath := filepath.Join(baseDir, filepath.Clean(archivePath))

	// Security check
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	absArchive, err := filepath.Abs(fullPath)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if !strings.HasPrefix(absArchive, absBase) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Check if path exists
	info, err := os.Stat(absArchive)
	if err != nil {
		http.Error(w, "Path not found", http.StatusNotFound)
		return
	}

	// Determine archive name
	archiveName := "archive.zip"
	if info.IsDir() {
		archiveName = filepath.Base(absArchive) + ".zip"
	} else {
		archiveName = strings.TrimSuffix(filepath.Base(absArchive), filepath.Ext(absArchive)) + ".zip"
	}

	// Set headers for download
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", archiveName))

	// Create zip writer
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	if info.IsDir() {
		// Archive directory
		err = h.archiveDirectory(zipWriter, absArchive, filepath.Base(absArchive))
	} else {
		// Archive single file
		err = h.archiveFile(zipWriter, absArchive, filepath.Base(absArchive))
	}

	if err != nil {
		log.Printf("Archive error: %v", err)
		return
	}

	log.Printf("Created archive: %s (%s)", archiveName, archivePath)
}

// archiveDirectory adds a directory to the zip archive
func (h *Handler) archiveDirectory(zipWriter *zip.Writer, dirPath, basePath string) error {
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}

		// Create zip path
		zipPath := filepath.Join(basePath, relPath)
		
		// Skip the root directory itself
		if path == dirPath {
			return nil
		}

		if info.IsDir() {
			// Add directory entry
			_, err := zipWriter.Create(zipPath + "/")
			return err
		}

		// Add file
		return h.addFileToZip(zipWriter, path, zipPath)
	})
}

// archiveFile adds a single file to the zip archive
func (h *Handler) archiveFile(zipWriter *zip.Writer, filePath, zipPath string) error {
	return h.addFileToZip(zipWriter, filePath, zipPath)
}

// addFileToZip adds a file to the zip archive
func (h *Handler) addFileToZip(zipWriter *zip.Writer, filePath, zipPath string) error {
	// Open source file
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Get file info
	info, err := file.Stat()
	if err != nil {
		return err
	}

	// Create zip header
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}

	header.Name = filepath.ToSlash(zipPath)
	header.Method = zip.Deflate

	// Create writer for file
	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	// Copy file content
	_, err = io.Copy(writer, file)
	return err
}
