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
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
    <title>%s</title>
    <style>
        * { 
            box-sizing: border-box;
            -webkit-tap-highlight-color: transparent;
            margin: 0;
            padding: 0;
        }
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', 'Helvetica Neue', Arial, sans-serif; 
            margin: 0; 
            padding: 0;
            background: #f8f9fa; 
            -webkit-font-smoothing: antialiased;
            -moz-osx-font-smoothing: grayscale;
            color: #1e2939;
        }
        .header { 
            background: white; 
            padding: 20px; 
            box-shadow: 0 1px 3px rgba(30, 41, 57, 0.08);
            position: sticky;
            top: 0;
            z-index: 100;
            border-bottom: 1px solid #e8eaed;
        }
        h1 { 
            color: #1e2939; 
            margin: 0 0 20px 0; 
            font-size: 20px;
            font-weight: 700;
            word-break: break-word;
            display: flex;
            align-items: center;
            gap: 10px;
            letter-spacing: -0.02em;
        }
        .toolbar { 
            display: grid;
            grid-template-columns: 1fr auto auto auto;
            gap: 10px;
            margin-bottom: 0;
        }
        .search-box { 
            padding: 12px 16px;
            border: 2px solid #e8eaed; 
            border-radius: 4px; 
            font-size: 15px;
            background: white;
            transition: all 0.2s ease;
            font-family: inherit;
            color: #1e2939;
        }
        .search-box:focus {
            outline: none;
            border-color: #1e2939;
            box-shadow: 0 0 0 3px rgba(30, 41, 57, 0.08);
        }
        .btn { 
            background: white; 
            color: #1e2939; 
            border: 2px solid #e8eaed; 
            padding: 12px 16px;
            border-radius: 4px; 
            cursor: pointer; 
            font-size: 18px;
            font-weight: 600;
            text-decoration: none; 
            display: flex;
            align-items: center;
            justify-content: center;
            min-width: 50px;
            min-height: 50px;
            transition: all 0.15s ease;
            touch-action: manipulation;
            gap: 0;
        }
        .btn-text {
            display: none;
        }
        .btn:hover { 
            background: #1e2939;
            color: white;
            border-color: #1e2939;
        }
        .btn:active { 
            background: #0d1520;
            border-color: #0d1520;
            color: white;
            transform: scale(0.98);
        }
        .upload-area { 
            display: none; 
            background: #f8f9fa; 
            padding: 28px; 
            border-radius: 4px; 
            margin-top: 20px; 
            border: 2px dashed #c5c9cf; 
            text-align: center;
            transition: all 0.3s ease;
        }
        .upload-area.drag-over { 
            background: #e8eaed; 
            border-color: #1e2939;
            border-width: 2px;
        }
        .upload-area h3 {
            margin: 0 0 10px 0;
            font-size: 18px;
            color: #1e2939;
            font-weight: 600;
        }
        .upload-area p {
            margin: 0 0 18px 0;
            color: #6c757d;
            font-size: 14px;
        }
        input[type="file"] { 
            margin: 12px 0;
            padding: 12px;
            width: 100%%;
            font-size: 14px;
            border: 1px solid #e8eaed;
            border-radius: 4px;
            background: white;
            font-family: inherit;
        }
        .upload-btn {
            width: 100%%;
            padding: 16px;
            font-size: 16px;
            margin-top: 10px;
            font-weight: 600;
        }
        ul { 
            list-style: none; 
            padding: 0; 
            margin: 0;
        }
        li { 
            padding: 16px 20px; 
            border-bottom: 1px solid #e8eaed; 
            background: white;
            display: grid;
            grid-template-columns: 1fr auto;
            align-items: center;
            gap: 16px;
            min-height: 68px;
            transition: all 0.2s ease;
        }
        li:hover { background: #f8f9fa; }
        li:active { 
            background: #e8eaed;
            transform: scale(0.998);
        }
        li:last-child { border-bottom: none; }
        a { 
            text-decoration: none; 
            color: #1e2939; 
            word-break: break-word;
            line-height: 1.5;
            transition: all 0.15s ease;
            font-weight: 500;
        }
        a:hover { 
            color: #2a3d54;
        }
        a:active { opacity: 0.7; }
        .dir { 
            font-weight: 600;
            color: #1e2939;
        }
        .file { 
            color: #495057;
            font-weight: 500;
        }
        .item-info { 
            min-width: 0;
            display: flex;
            align-items: center;
            gap: 14px;
            font-size: 15px;
            overflow: hidden;
        }
        .item-icon {
            font-size: 28px;
            flex-shrink: 0;
            line-height: 1;
            filter: grayscale(0.2);
        }
        .item-name {
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
            flex: 1;
            min-width: 0;
        }
        .item-actions { 
            display: flex; 
            gap: 10px;
            flex-shrink: 0;
        }
        .action-btn {
            background: white;
            color: #1e2939;
            border: 2px solid #e8eaed;
            padding: 0;
            border-radius: 4px;
            cursor: pointer;
            font-size: 18px;
            font-weight: 600;
            text-decoration: none;
            display: flex;
            align-items: center;
            justify-content: center;
            min-width: 46px;
            min-height: 46px;
            transition: all 0.15s ease;
            touch-action: manipulation;
        }
        .action-btn:hover { 
            background: #1e2939;
            color: white;
            border-color: #1e2939;
        }
        .action-btn:active { 
            background: #0d1520;
            border-color: #0d1520;
            transform: scale(0.96);
        }
        .clipboard-modal { 
            display: none; 
            position: fixed; 
            top: 0; 
            left: 0; 
            width: 100%%; 
            height: 100%%; 
            background: rgba(30, 41, 57, 0.75); 
            z-index: 1000;
            animation: fadeIn 0.25s ease;
            backdrop-filter: blur(4px);
        }
        @keyframes fadeIn {
            from { opacity: 0; }
            to { opacity: 1; }
        }
        .clipboard-content { 
            position: fixed;
            bottom: 0;
            left: 0;
            right: 0;
            background: white; 
            padding: 24px;
            padding-bottom: calc(24px + env(safe-area-inset-bottom));
            border-radius: 0;
            max-height: 90vh;
            overflow-y: auto;
            animation: slideUp 0.3s cubic-bezier(0.4, 0, 0.2, 1);
            box-shadow: 0 -8px 32px rgba(30, 41, 57, 0.2);
        }
        @keyframes slideUp {
            from { transform: translateY(100%%); opacity: 0; }
            to { transform: translateY(0); opacity: 1; }
        }
        .clipboard-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 20px;
            padding-bottom: 16px;
            border-bottom: 2px solid #e8eaed;
        }
        .clipboard-content h2 {
            margin: 0;
            font-size: 22px;
            font-weight: 700;
            color: #1e2939;
            letter-spacing: -0.02em;
        }
        .clipboard-content textarea { 
            width: 100%%; 
            min-height: 200px;
            padding: 16px; 
            border: 2px solid #e8eaed; 
            border-radius: 4px; 
            font-family: 'SF Mono', 'Monaco', 'Menlo', 'Courier New', monospace;
            font-size: 14px;
            resize: vertical;
            margin-bottom: 14px;
            background: white;
            color: #1e2939;
            transition: all 0.2s ease;
            line-height: 1.6;
        }
        .clipboard-content textarea:focus {
            outline: none;
            border-color: #1e2939;
            box-shadow: 0 0 0 3px rgba(30, 41, 57, 0.08);
        }
        .clipboard-buttons {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 12px;
            margin-bottom: 20px;
        }
        .clipboard-items { 
            max-height: 320px; 
            overflow-y: auto;
            margin-top: 20px;
            -webkit-overflow-scrolling: touch;
        }
        .clipboard-items h3 {
            font-size: 17px;
            margin: 0 0 14px 0;
            color: #495057;
            font-weight: 600;
        }
        .clipboard-item { 
            background: #f8f9fa; 
            padding: 16px; 
            margin: 10px 0; 
            border-radius: 4px; 
            cursor: pointer; 
            border: 2px solid #e8eaed;
            word-break: break-word;
            transition: all 0.2s ease;
        }
        .clipboard-item:hover {
            background: white;
            border-color: #1e2939;
            box-shadow: 0 2px 8px rgba(30, 41, 57, 0.1);
        }
        .clipboard-item:active { 
            background: #e8eaed;
            transform: scale(0.99);
            box-shadow: none;
        }
        .clipboard-item small {
            display: block;
            color: #6c757d;
            margin-bottom: 8px;
            font-size: 12px;
            font-weight: 500;
        }
        .clipboard-item code {
            display: block;
            color: #1e2939;
            font-size: 13px;
            line-height: 1.5;
            font-family: inherit;
        }
        .close-btn { 
            font-size: 34px;
            cursor: pointer; 
            color: #6c757d;
            line-height: 1;
            padding: 10px;
            margin: -10px;
            min-width: 50px;
            min-height: 50px;
            display: flex;
            align-items: center;
            justify-content: center;
            touch-action: manipulation;
            border-radius: 4px;
            transition: all 0.2s ease;
        }
        .close-btn:hover {
            background: #f8f9fa;
            color: #1e2939;
        }
        .close-btn:active { 
            background: #e8eaed;
            transform: scale(0.94);
        }
        #search-results { 
            display: none; 
            background: white; 
            padding: 20px;
            margin-top: 20px;
            border-radius: 4px;
            box-shadow: 0 2px 12px rgba(30, 41, 57, 0.08);
            border: 1px solid #e8eaed;
        }
        #search-results h3 {
            margin: 0 0 14px 0;
            font-size: 17px;
            font-weight: 600;
            color: #1e2939;
        }
        #search-results ul {
            padding-left: 0;
        }
        #search-results li {
            padding: 14px;
            min-height: auto;
            border-radius: 4px;
            margin-bottom: 6px;
        }
        #search-results li:last-child {
            margin-bottom: 0;
        }

        /* Desktop/Tablet optimizations */
        @media (min-width: 769px) {
            body { 
                padding: 40px 80px; 
                background: #f8f9fa;
            }
            .header { 
                border-radius: 0;
                position: static;
                padding: 36px 40px;
                max-width: 1400px;
                margin: 0 auto 2px;
                box-shadow: none;
                border-bottom: 2px solid #e8eaed;
            }
            h1 { 
                font-size: 28px;
                margin-bottom: 28px;
            }
            .toolbar {
                grid-template-columns: 1fr auto auto auto;
                gap: 16px;
            }
            .search-box {
                font-size: 15px;
                padding: 14px 18px;
            }
            .btn {
                min-width: 120px;
                min-height: 48px;
                font-size: 15px;
                padding: 14px 20px;
                gap: 8px;
            }
            .btn-text {
                display: inline;
                font-weight: 600;
            }
            ul {
                max-width: 1400px;
                margin: 0 auto;
                border-radius: 0;
                overflow: visible;
                box-shadow: none;
                background: white;
            }
            li { 
                border-radius: 0;
                margin-bottom: 0;
                border-bottom: 1px solid #e8eaed;
                padding: 20px 40px;
                min-height: 72px;
            }
            li:first-child {
                border-radius: 0;
                border-top: none;
            }
            li:last-child {
                border-radius: 0;
                border-bottom: 2px solid #e8eaed;
            }
            li:hover { 
                background: #f8f9fa;
                border-left: 3px solid #1e2939;
                padding-left: 37px;
            }
            li:active { 
                background: #e8eaed;
            }
            .item-info {
                font-size: 16px;
                gap: 16px;
            }
            .item-icon {
                font-size: 30px;
            }
            .action-btn {
                min-width: 44px;
                min-height: 44px;
                font-size: 16px;
            }
            .clipboard-content {
                position: absolute;
                top: 50%%;
                left: 50%%;
                transform: translate(-50%%, -50%%);
                bottom: auto;
                right: auto;
                width: 90%%;
                max-width: 700px;
                border-radius: 0;
                animation: scaleIn 0.25s cubic-bezier(0.4, 0, 0.2, 1);
                padding: 32px;
            }
            @keyframes scaleIn {
                from { transform: translate(-50%%, -50%%) scale(0.95); opacity: 0; }
                to { transform: translate(-50%%, -50%%) scale(1); opacity: 1; }
            }
        }

        /* Large desktop */
        @media (min-width: 1600px) {
            body {
                padding: 50px 120px;
            }
            .header {
                padding: 40px 48px;
            }
            h1 {
                font-size: 30px;
            }
            li {
                padding: 24px 48px;
            }
            li:hover {
                padding-left: 45px;
            }
            .item-info {
                font-size: 17px;
            }
        }

        /* Small mobile phones */
        @media (max-width: 375px) {
            .header { padding: 16px; }
            h1 { font-size: 18px; }
            .btn { 
                padding: 10px;
                font-size: 18px;
                min-width: 46px;
                min-height: 46px;
            }
            li { padding: 14px 16px; }
            .item-info { 
                font-size: 14px;
                gap: 12px;
            }
            .item-icon {
                font-size: 24px;
            }
            .action-btn {
                min-width: 44px;
                min-height: 44px;
                font-size: 18px;
            }
        }
    </style>
</head>
<body>
    <div class="header">
        <h1><span>📁</span><span>%s</span></h1>
        <div class="toolbar">
            <input type="text" id="searchBox" class="search-box" placeholder="Search files..." autocomplete="off">
            <button class="btn" onclick="toggleUpload()" title="Upload">
                <span>⬆️</span>
                <span class="btn-text">Upload</span>
            </button>
            <button class="btn" onclick="openClipboard()" title="Clipboard">
                <span>📋</span>
                <span class="btn-text">Clipboard</span>
            </button>
            <a href="/api/archive?path=%s" class="btn" title="Download ZIP">
                <span>⬇️</span>
                <span class="btn-text">Download</span>
            </a>
        </div>
        <div id="uploadArea" class="upload-area">
            <h3>📤 Upload Files</h3>
            <p>Tap to select files or drag and drop</p>
            <input type="file" id="fileInput" multiple>
            <button class="btn upload-btn" onclick="uploadFiles()">Upload</button>
        </div>
        <div id="search-results"></div>
    </div>
    <ul id="file-list">`, urlPath, urlPath, urlPath)
	
	// Parent directory link
	if urlPath != "/" {
		fmt.Fprintf(w, `<li>
			<div class="item-info">
				<span class="item-icon">📁</span>
				<a href=".." class="dir item-name">..</a>
			</div>
			<div class="item-actions"></div>
		</li>`)
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
				<div class="item-info">
					<span class="item-icon">%s</span>
					<a href="%s" class="%s item-name">%s</a>
				</div>
				<div class="item-actions">
					<a href="/api/archive?path=%s" class="action-btn" title="Download as ZIP">⬇️</a>
				</div>
			</li>`, icon, href, class, name, href)
		} else {
			// For files, only show download button
			downloadHref := href + "?download=1"
			
			fmt.Fprintf(w, `<li>
				<div class="item-info">
					<span class="item-icon">%s</span>
					<a href="%s" class="%s item-name">%s</a>
				</div>
				<div class="item-actions">
					<a href="%s" class="action-btn" title="Download">⬇️</a>
				</div>
			</li>`, icon, href, class, name, downloadHref)
		}
	}
	
	fmt.Fprintf(w, `
    </ul>
    
    <!-- Clipboard Modal -->
    <div id="clipboardModal" class="clipboard-modal">
        <div class="clipboard-content">
            <div class="clipboard-header">
                <h2>📋 Clipboard Sharing</h2>
                <span class="close-btn" onclick="closeClipboard()">&times;</span>
            </div>
            <textarea id="clipboardText" placeholder="Paste or type text here..."></textarea>
            <div class="clipboard-buttons">
                <button class="btn" onclick="saveClipboard()">💾 Save</button>
                <button class="btn" onclick="loadClipboard()">📥 Load</button>
            </div>
            <div id="clipboardItems" class="clipboard-items"></div>
        </div>
    </div>

    <script>
        const currentPath = %q;
        
        // Upload functionality
        function toggleUpload() {
            const area = document.getElementById('uploadArea');
            area.style.display = area.style.display === 'none' ? 'block' : 'none';
        }

        const uploadArea = document.getElementById('uploadArea');
        const fileInput = document.getElementById('fileInput');

        uploadArea.addEventListener('dragover', (e) => {
            e.preventDefault();
            uploadArea.classList.add('drag-over');
        });

        uploadArea.addEventListener('dragleave', () => {
            uploadArea.classList.remove('drag-over');
        });

        uploadArea.addEventListener('drop', (e) => {
            e.preventDefault();
            uploadArea.classList.remove('drag-over');
            fileInput.files = e.dataTransfer.files;
        });

        uploadArea.addEventListener('click', (e) => {
            if (e.target === uploadArea) {
                fileInput.click();
            }
        });

        async function uploadFiles() {
            const files = fileInput.files;
            if (files.length === 0) {
                alert('Please select files to upload');
                return;
            }

            const formData = new FormData();
            formData.append('path', currentPath);
            for (let file of files) {
                formData.append('files', file);
            }

            try {
                const response = await fetch('/api/upload', {
                    method: 'POST',
                    body: formData
                });
                const result = await response.json();
                
                if (response.ok) {
                    alert('Upload successful: ' + result.count + ' files uploaded');
                    location.reload();
                } else {
                    alert('Upload failed: ' + (result.error || 'Unknown error'));
                }
            } catch (error) {
                alert('Upload failed: ' + error.message);
            }
        }

        // Search functionality
        let searchTimeout;
        document.getElementById('searchBox').addEventListener('input', (e) => {
            clearTimeout(searchTimeout);
            const query = e.target.value.trim();
            
            if (query.length < 2) {
                document.getElementById('search-results').style.display = 'none';
                return;
            }

            searchTimeout = setTimeout(async () => {
                try {
                    const response = await fetch('/api/search?q=' + encodeURIComponent(query) + '&path=' + currentPath);
                    const data = await response.json();
                    displaySearchResults(data);
                } catch (error) {
                    console.error('Search failed:', error);
                }
            }, 300);
        });

        function displaySearchResults(data) {
            const resultsDiv = document.getElementById('search-results');
            if (data.count === 0) {
                resultsDiv.innerHTML = '<p>No results found</p>';
            } else {
                let html = '<h3>🔍 Search Results (' + data.count + ')</h3><ul style="list-style: none; padding: 0;">';
                for (let item of data.results) {
                    const icon = item.is_dir ? '📁' : '📄';
                    html += '<li style="padding: 8px; border-bottom: 1px solid #ddd;"><a href="' + item.path + '">' + icon + ' ' + item.name + '</a> <small style="color: #999;">' + item.path + '</small></li>';
                }
                html += '</ul>';
                resultsDiv.innerHTML = html;
            }
            resultsDiv.style.display = 'block';
        }

        // Clipboard functionality
        function openClipboard() {
            document.getElementById('clipboardModal').style.display = 'block';
            // Don't auto-load on open to prevent interrupting user typing
        }

        function closeClipboard() {
            document.getElementById('clipboardModal').style.display = 'none';
        }

        async function saveClipboard() {
            const content = document.getElementById('clipboardText').value;
            if (!content) {
                alert('Please enter some text');
                return;
            }

            try {
                const response = await fetch('/api/clipboard', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ content: content, ttl: 60 })
                });
                
                if (response.ok) {
                    alert('Saved to clipboard!');
                    document.getElementById('clipboardText').value = '';
                    // Only refresh after saving
                    loadClipboard();
                } else {
                    alert('Failed to save');
                }
            } catch (error) {
                alert('Error: ' + error.message);
            }
        }

        async function loadClipboard() {
            try {
                const response = await fetch('/api/clipboard');
                const data = await response.json();
                
                const itemsDiv = document.getElementById('clipboardItems');
                if (data.count === 0) {
                    itemsDiv.innerHTML = '<p>No saved clipboard items</p>';
                } else {
                    let html = '<h3>Saved Items (' + data.count + ')</h3>';
                    for (let item of data.items) {
                        const preview = item.content.substring(0, 100) + (item.content.length > 100 ? '...' : '');
                        html += '<div class="clipboard-item" onclick="useClipboardItem(\'' + item.id + '\')">';
                        html += '<small>' + new Date(item.created_at).toLocaleString() + '</small><br>';
                        html += '<code>' + escapeHtml(preview) + '</code>';
                        html += '</div>';
                    }
                    itemsDiv.innerHTML = html;
                }
            } catch (error) {
                console.error('Failed to load clipboard:', error);
            }
        }

        async function useClipboardItem(id) {
            try {
                const response = await fetch('/api/clipboard?id=' + id);
                const item = await response.json();
                document.getElementById('clipboardText').value = item.content;
            } catch (error) {
                alert('Failed to load item: ' + error.message);
            }
        }

        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        // Close modal when clicking outside
        window.onclick = function(event) {
            const modal = document.getElementById('clipboardModal');
            if (event.target === modal) {
                closeClipboard();
            }
        }
    </script>
    <script src="/__watcher.js"></script>
</body>
</html>`, urlPath)
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
