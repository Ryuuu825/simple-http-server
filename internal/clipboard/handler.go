package clipboard

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"
)

// ClipItem represents a clipboard item
type ClipItem struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Handler manages clipboard sharing
type Handler struct {
	mu        sync.RWMutex
	clipboard map[string]*ClipItem
}

// NewHandler creates a new clipboard handler
func NewHandler() *Handler {
	h := &Handler{
		clipboard: make(map[string]*ClipItem),
	}
	
	// Start cleanup goroutine
	go h.cleanupExpired()
	
	return h
}

// ServeHTTP handles clipboard requests
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getClipboard(w, r)
	case http.MethodPost:
		h.setClipboard(w, r)
	case http.MethodDelete:
		h.clearClipboard(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getClipboard retrieves the current clipboard content
func (h *Handler) getClipboard(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	
	h.mu.RLock()
	defer h.mu.RUnlock()

	if id != "" {
		// Get specific item
		item, exists := h.clipboard[id]
		if !exists || time.Now().After(item.ExpiresAt) {
			http.Error(w, "Clipboard item not found or expired", http.StatusNotFound)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(item)
		return
	}

	// Get all non-expired items
	items := []*ClipItem{}
	for _, item := range h.clipboard {
		if time.Now().Before(item.ExpiresAt) {
			items = append(items, item)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"items": items,
		"count": len(items),
	})
}

// setClipboard saves content to clipboard
func (h *Handler) setClipboard(w http.ResponseWriter, r *http.Request) {
	// Read request body
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}

	var req struct {
		Content string `json:"content"`
		TTL     int    `json:"ttl"` // Time to live in minutes (default: 60)
	}

	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Content == "" {
		http.Error(w, "Content is required", http.StatusBadRequest)
		return
	}

	// Default TTL: 60 minutes
	ttl := 60
	if req.TTL > 0 && req.TTL <= 1440 { // Max 24 hours
		ttl = req.TTL
	}

	// Create clipboard item
	now := time.Now()
	item := &ClipItem{
		ID:        generateID(),
		Content:   req.Content,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Duration(ttl) * time.Minute),
	}

	h.mu.Lock()
	h.clipboard[item.ID] = item
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(item)
}

// clearClipboard removes clipboard content
func (h *Handler) clearClipboard(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	
	h.mu.Lock()
	defer h.mu.Unlock()

	if id != "" {
		// Delete specific item
		if _, exists := h.clipboard[id]; !exists {
			http.Error(w, "Clipboard item not found", http.StatusNotFound)
			return
		}
		delete(h.clipboard, id)
	} else {
		// Clear all
		h.clipboard = make(map[string]*ClipItem)
	}

	w.WriteHeader(http.StatusNoContent)
}

// cleanupExpired removes expired clipboard items
func (h *Handler) cleanupExpired() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		h.mu.Lock()
		now := time.Now()
		for id, item := range h.clipboard {
			if now.After(item.ExpiresAt) {
				delete(h.clipboard, id)
			}
		}
		h.mu.Unlock()
	}
}

// generateID generates a simple unique ID
func generateID() string {
	return time.Now().Format("20060102150405")
}
