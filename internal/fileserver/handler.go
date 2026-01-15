package fileserver

import (
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"simple.http.server/internal/config"
)

//go:embed watcher-client.js
var watcherClientJS string

// FileServer handles static file serving
type FileServer struct {
	mu        sync.RWMutex
	clients   map[chan string]bool
	config    *config.Config
}

// NewFileServer creates a new file server instance
func NewFileServer(cfg *config.Config) *FileServer {
	fs := &FileServer{
		clients: make(map[chan string]bool),
		config:  cfg,
	}
	
	// Start file watcher
	go fs.watchFiles()
	
	return fs
}

// ServeHTTP serves static files
func (fs *FileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Serve embedded JavaScript file
	if r.URL.Path == "/__watcher.js" {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write([]byte(watcherClientJS))
		return
	}
	
	dir := fs.config.GetFileServerDir()
	
	// Security: prevent directory traversal
	cleanPath := filepath.Clean(r.URL.Path)
	fullPath := filepath.Join(dir, cleanPath)
	
	// Check if path is within the allowed directory
	absDir, err := filepath.Abs(dir)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	if !filepath.HasPrefix(absPath, absDir) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	
	// Check if file exists
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	// If directory, serve index
	if info.IsDir() {
		fs.serveDirectory(w, r, fullPath, cleanPath)
		return
	}
	
	// Check if download is requested
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(fullPath)))
	}
	
	// Serve file
	http.ServeFile(w, r, fullPath)
}

// serveDirectory generates a directory listing
func (fs *FileServer) serveDirectory(w http.ResponseWriter, r *http.Request, fullPath, urlPath string) {
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		http.Error(w, "Unable to read directory", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Directory: %s</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background: #f5f5f5; }
        h1 { color: #333; }
        ul { list-style: none; padding: 0; }
        li { 
            padding: 12px; 
            border-bottom: 1px solid #ddd; 
            background: white;
            margin-bottom: 2px;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        li:hover { background: #f9f9f9; }
        a { text-decoration: none; color: #0066cc; }
        a:hover { text-decoration: underline; }
        .dir { font-weight: bold; }
        .file { color: #666; }
        .item-info { flex: 1; }
        .download-btn {
            background: #3498db;
            color: white;
            border: none;
            padding: 6px 12px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 12px;
            text-decoration: none;
            display: inline-block;
        }
        .download-btn:hover {
            background: #2980b9;
        }
    </style>
</head>
<body>
    <h1>Index of %s</h1>
    <ul id="file-list">`, urlPath, urlPath)
	
	// Parent directory link
	if urlPath != "/" {
		fmt.Fprintf(w, `<li><a href=".." class="dir">📁 ..</a></li>`)
	}
	
	for _, entry := range entries {
		name := entry.Name()
		icon := "📄"
		class := "file"
		href := filepath.Join(urlPath, name)
		
		if entry.IsDir() {
			icon = "📁"
			class = "dir"
			href += "/"
			fmt.Fprintf(w, `<li>
				<div class="item-info"><a href="%s" class="%s">%s %s</a></div>
			</li>`, href, class, icon, name)
		} else {
			// For files, add both view and download options
			downloadHref := href + "?download=1"
			fmt.Fprintf(w, `<li>
				<div class="item-info"><a href="%s" class="%s">%s %s</a></div>
				<a href="%s" class="download-btn">⬇ Download</a>
			</li>`, href, class, icon, name, downloadHref)
		}
	}
	
	fmt.Fprintf(w, `
    </ul>
    <script src="/__watcher.js"></script>
</body>
</html>`)
}

// HandleSSE handles Server-Sent Events for file updates
func (fs *FileServer) HandleSSE(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Accel-Buffering", "no")
	
	// Check if response writer supports flushing
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}
	
	// Create a channel for this client
	clientChan := make(chan string, 10)
	
	// Register client
	fs.mu.Lock()
	fs.clients[clientChan] = true
	fs.mu.Unlock()
	
	log.Printf("SSE client connected from %s", r.RemoteAddr)
	
	// Remove client on disconnect
	defer func() {
		fs.mu.Lock()
		delete(fs.clients, clientChan)
		fs.mu.Unlock()
		log.Printf("SSE client disconnected from %s", r.RemoteAddr)
	}()
	
	// Send initial connection message
	fmt.Fprintf(w, "data: Connected to file watcher\n\n")
	flusher.Flush()
	
	// Keep-alive ticker to prevent timeout
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	
	// Listen for messages
	for {
		select {
		case msg, ok := <-clientChan:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
			
		case <-ticker.C:
			// Send keep-alive comment
			fmt.Fprintf(w, ": keep-alive\n\n")
			flusher.Flush()
			
		case <-r.Context().Done():
			return
		}
	}
}

// BroadcastChange sends a change notification to all connected clients
func (fs *FileServer) BroadcastChange(message string) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	
	log.Printf("Broadcasting change: %s to %d clients", message, len(fs.clients))
	
	for clientChan := range fs.clients {
		select {
		case clientChan <- message:
		default:
			// Client channel is full, skip
		}
	}
}
