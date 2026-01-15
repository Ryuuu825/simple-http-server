package preview

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"simple.http.server/internal/config"
)

// Handler manages file preview
type Handler struct {
	config *config.Config
}

// NewHandler creates a new preview handler
func NewHandler(cfg *config.Config) *Handler {
	return &Handler{config: cfg}
}

// ServeHTTP handles preview requests
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get file path
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "Path parameter is required", http.StatusBadRequest)
		return
	}

	// Get base directory
	baseDir := h.config.GetFileServerDir()
	fullPath := filepath.Join(baseDir, filepath.Clean(filePath))

	// Security check
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	absFile, err := filepath.Abs(fullPath)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if !strings.HasPrefix(absFile, absBase) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Check if file exists
	info, err := os.Stat(absFile)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	if info.IsDir() {
		http.Error(w, "Cannot preview directory", http.StatusBadRequest)
		return
	}

	// Determine file type and serve preview
	ext := strings.ToLower(filepath.Ext(absFile))
	
	switch {
	case isImage(ext):
		h.serveImagePreview(w, r, absFile, info)
	case isVideo(ext):
		h.serveVideoPreview(w, r, absFile, filePath)
	case isAudio(ext):
		h.serveAudioPreview(w, r, absFile, filePath)
	case isCode(ext):
		h.serveCodePreview(w, r, absFile, ext)
	case ext == ".pdf":
		h.servePDFPreview(w, r, absFile, filePath)
	case isText(ext):
		h.serveTextPreview(w, r, absFile)
	default:
		http.Error(w, "Preview not supported for this file type", http.StatusBadRequest)
	}
}

// serveImagePreview serves image preview HTML
func (h *Handler) serveImagePreview(w http.ResponseWriter, r *http.Request, filePath string, info os.FileInfo) {
	fileName := filepath.Base(filePath)
	fileSize := formatFileSize(info.Size())
	
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Preview: %s</title>
    <style>
        body { margin: 0; padding: 20px; background: #1a1a1a; color: #fff; font-family: Arial, sans-serif; display: flex; flex-direction: column; align-items: center; }
        .info { margin-bottom: 20px; }
        img { max-width: 100%%; max-height: 80vh; box-shadow: 0 4px 6px rgba(0,0,0,0.3); }
        .back-btn { background: #3498db; color: white; padding: 10px 20px; text-decoration: none; border-radius: 4px; }
    </style>
</head>
<body>
    <div class="info">
        <h2>📷 %s</h2>
        <p>Size: %s</p>
        <a href="javascript:history.back()" class="back-btn">← Back</a>
    </div>
    <img src="%s" alt="%s">
</body>
</html>`, fileName, fileName, fileSize, r.URL.Query().Get("path"), fileName)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// serveVideoPreview serves video preview HTML
func (h *Handler) serveVideoPreview(w http.ResponseWriter, r *http.Request, filePath, urlPath string) {
	fileName := filepath.Base(filePath)
	
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Preview: %s</title>
    <style>
        body { margin: 0; padding: 20px; background: #1a1a1a; color: #fff; font-family: Arial, sans-serif; display: flex; flex-direction: column; align-items: center; }
        .info { margin-bottom: 20px; }
        video { max-width: 100%%; max-height: 80vh; }
        .back-btn { background: #3498db; color: white; padding: 10px 20px; text-decoration: none; border-radius: 4px; }
    </style>
</head>
<body>
    <div class="info">
        <h2>🎬 %s</h2>
        <a href="javascript:history.back()" class="back-btn">← Back</a>
    </div>
    <video controls autoplay>
        <source src="%s" type="video/mp4">
        Your browser does not support video playback.
    </video>
</body>
</html>`, fileName, fileName, urlPath)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// serveAudioPreview serves audio preview HTML
func (h *Handler) serveAudioPreview(w http.ResponseWriter, r *http.Request, filePath, urlPath string) {
	fileName := filepath.Base(filePath)
	
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Preview: %s</title>
    <style>
        body { margin: 0; padding: 20px; background: #1a1a1a; color: #fff; font-family: Arial, sans-serif; display: flex; flex-direction: column; align-items: center; }
        .info { margin-bottom: 20px; text-align: center; }
        audio { width: 500px; max-width: 100%%; }
        .back-btn { background: #3498db; color: white; padding: 10px 20px; text-decoration: none; border-radius: 4px; }
    </style>
</head>
<body>
    <div class="info">
        <h2>🎵 %s</h2>
        <p><a href="javascript:history.back()" class="back-btn">← Back</a></p>
    </div>
    <audio controls autoplay>
        <source src="%s">
        Your browser does not support audio playback.
    </audio>
</body>
</html>`, fileName, fileName, urlPath)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// serveCodePreview serves code preview with syntax highlighting
func (h *Handler) serveCodePreview(w http.ResponseWriter, r *http.Request, filePath, ext string) {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	fileName := filepath.Base(filePath)
	language := getLanguage(ext)
	
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Preview: %s</title>
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/styles/github-dark.min.css">
    <script src="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/highlight.min.js"></script>
    <style>
        body { margin: 0; padding: 20px; background: #0d1117; color: #c9d1d9; font-family: Arial, sans-serif; }
        .header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
        .back-btn { background: #3498db; color: white; padding: 10px 20px; text-decoration: none; border-radius: 4px; }
        pre { margin: 0; padding: 20px; background: #161b22; border-radius: 6px; overflow-x: auto; }
        code { font-family: 'Monaco', 'Menlo', 'Courier New', monospace; font-size: 14px; }
    </style>
</head>
<body>
    <div class="header">
        <h2>📝 %s</h2>
        <a href="javascript:history.back()" class="back-btn">← Back</a>
    </div>
    <pre><code class="language-%s">%s</code></pre>
    <script>hljs.highlightAll();</script>
</body>
</html>`, fileName, fileName, language, escapeHTML(string(content)))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// servePDFPreview serves PDF preview HTML
func (h *Handler) servePDFPreview(w http.ResponseWriter, r *http.Request, filePath, urlPath string) {
	fileName := filepath.Base(filePath)
	
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Preview: %s</title>
    <style>
        body { margin: 0; padding: 0; background: #333; }
        iframe { width: 100%%; height: 100vh; border: none; }
        .header { background: #1a1a1a; color: #fff; padding: 10px 20px; }
        .back-btn { background: #3498db; color: white; padding: 8px 16px; text-decoration: none; border-radius: 4px; }
    </style>
</head>
<body>
    <div class="header">
        <a href="javascript:history.back()" class="back-btn">← Back</a>
        <span style="margin-left: 20px;">📄 %s</span>
    </div>
    <iframe src="%s"></iframe>
</body>
</html>`, fileName, fileName, urlPath)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// serveTextPreview serves plain text preview
func (h *Handler) serveTextPreview(w http.ResponseWriter, r *http.Request, filePath string) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	fileName := filepath.Base(filePath)
	
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Preview: %s</title>
    <style>
        body { margin: 0; padding: 20px; background: #1a1a1a; color: #c9d1d9; font-family: Arial, sans-serif; }
        .header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
        .back-btn { background: #3498db; color: white; padding: 10px 20px; text-decoration: none; border-radius: 4px; }
        pre { background: #0d1117; padding: 20px; border-radius: 6px; overflow-x: auto; white-space: pre-wrap; word-wrap: break-word; }
    </style>
</head>
<body>
    <div class="header">
        <h2>📄 %s</h2>
        <a href="javascript:history.back()" class="back-btn">← Back</a>
    </div>
    <pre>%s</pre>
</body>
</html>`, fileName, fileName, escapeHTML(string(content)))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// Helper functions

func isImage(ext string) bool {
	images := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg", ".ico"}
	for _, img := range images {
		if ext == img {
			return true
		}
	}
	return false
}

func isVideo(ext string) bool {
	videos := []string{".mp4", ".webm", ".ogg", ".mov", ".avi", ".mkv"}
	for _, vid := range videos {
		if ext == vid {
			return true
		}
	}
	return false
}

func isAudio(ext string) bool {
	audios := []string{".mp3", ".wav", ".ogg", ".m4a", ".flac", ".aac"}
	for _, aud := range audios {
		if ext == aud {
			return true
		}
	}
	return false
}

func isCode(ext string) bool {
	codes := []string{".go", ".js", ".ts", ".py", ".java", ".c", ".cpp", ".h", ".hpp", ".cs", ".rb", ".php", ".swift", ".kt", ".rs", ".html", ".css", ".scss", ".json", ".xml", ".yaml", ".yml", ".toml", ".sql", ".sh", ".bash", ".ps1"}
	for _, code := range codes {
		if ext == code {
			return true
		}
	}
	return false
}

func isText(ext string) bool {
	texts := []string{".txt", ".md", ".log", ".csv", ".conf", ".ini", ".cfg"}
	for _, txt := range texts {
		if ext == txt {
			return true
		}
	}
	return false
}

func getLanguage(ext string) string {
	languages := map[string]string{
		".go":   "go",
		".js":   "javascript",
		".ts":   "typescript",
		".py":   "python",
		".java": "java",
		".c":    "c",
		".cpp":  "cpp",
		".cs":   "csharp",
		".rb":   "ruby",
		".php":  "php",
		".html": "html",
		".css":  "css",
		".json": "json",
		".xml":  "xml",
		".yaml": "yaml",
		".yml":  "yaml",
		".sql":  "sql",
		".sh":   "bash",
		".bash": "bash",
	}
	if lang, ok := languages[ext]; ok {
		return lang
	}
	return "plaintext"
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
