package admin

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"

	"simple.http.server/internal/config"
	"simple.http.server/internal/proxy"

	"github.com/google/uuid"
)

// Handler manages the admin panel API
type Handler struct {
	config       *config.Config
	proxyManager *proxy.ProxyManager
}

// NewHandler creates a new admin handler
func NewHandler(cfg *config.Config, pm *proxy.ProxyManager) *Handler {
	return &Handler{
		config:       cfg,
		proxyManager: pm,
	}
}

// ServeHTTP routes admin API requests
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/admin/api")

	switch {
	case path == "/proxies" && r.Method == http.MethodGet:
		h.listProxies(w, r)
	case path == "/proxies" && r.Method == http.MethodPost:
		h.addProxy(w, r)
	case strings.HasPrefix(path, "/proxies/") && r.Method == http.MethodPut:
		id := strings.TrimPrefix(path, "/proxies/")
		h.updateProxy(w, r, id)
	case strings.HasPrefix(path, "/proxies/") && r.Method == http.MethodDelete:
		id := strings.TrimPrefix(path, "/proxies/")
		h.deleteProxy(w, r, id)
	case path == "/settings/export" && r.Method == http.MethodGet:
		h.exportSettings(w, r)
	case path == "/settings/import" && r.Method == http.MethodPost:
		h.importSettings(w, r)
	case path == "/settings" && r.Method == http.MethodGet:
		h.getSettings(w, r)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

// listProxies returns all proxy rules
func (h *Handler) listProxies(w http.ResponseWriter, r *http.Request) {
	rules := h.config.GetProxyRules()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rules)
}

// addProxy adds a new proxy rule
func (h *Handler) addProxy(w http.ResponseWriter, r *http.Request) {
	var rule config.ProxyRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Generate ID if not provided
	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}

	// Validate - either PathPrefix or Port must be set
	if rule.PathPrefix == "" && rule.Port == 0 {
		http.Error(w, "Either PathPrefix or Port must be specified", http.StatusBadRequest)
		return
	}
	
	if rule.TargetURL == "" {
		http.Error(w, "TargetURL is required", http.StatusBadRequest)
		return
	}

	// Ensure PathPrefix starts with / if provided
	if rule.PathPrefix != "" && !strings.HasPrefix(rule.PathPrefix, "/") {
		rule.PathPrefix = "/" + rule.PathPrefix
	}

	h.config.AddProxyRule(rule)
	h.proxyManager.RefreshProxies()

	if rule.Port > 0 {
		log.Printf("Added port-based proxy rule: localhost:%d -> %s", rule.Port, rule.TargetURL)
	} else {
		log.Printf("Added path-based proxy rule: %s -> %s", rule.PathPrefix, rule.TargetURL)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rule)
}

// updateProxy updates an existing proxy rule
func (h *Handler) updateProxy(w http.ResponseWriter, r *http.Request, id string) {
	var rule config.ProxyRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate - either PathPrefix or Port must be set
	if rule.PathPrefix == "" && rule.Port == 0 {
		http.Error(w, "Either PathPrefix or Port must be specified", http.StatusBadRequest)
		return
	}
	
	if rule.TargetURL == "" {
		http.Error(w, "TargetURL is required", http.StatusBadRequest)
		return
	}

	// Ensure PathPrefix starts with / if provided
	if rule.PathPrefix != "" && !strings.HasPrefix(rule.PathPrefix, "/") {
		rule.PathPrefix = "/" + rule.PathPrefix
	}

	if !h.config.UpdateProxyRule(id, rule) {
		http.Error(w, "Proxy rule not found", http.StatusNotFound)
		return
	}

	h.proxyManager.RefreshProxies()

	log.Printf("Updated proxy rule: %s -> %s", rule.PathPrefix, rule.TargetURL)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rule)
}

// deleteProxy removes a proxy rule
func (h *Handler) deleteProxy(w http.ResponseWriter, r *http.Request, id string) {
	if !h.config.DeleteProxyRule(id) {
		http.Error(w, "Proxy rule not found", http.StatusNotFound)
		return
	}

	h.proxyManager.RefreshProxies()

	log.Printf("Deleted proxy rule: %s", id)

	w.WriteHeader(http.StatusNoContent)
}

// exportSettings exports current settings as JSON file
func (h *Handler) exportSettings(w http.ResponseWriter, r *http.Request) {
	data, err := h.config.ExportSettings()
	if err != nil {
		http.Error(w, "Failed to export settings", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=server-settings.json")
	w.Write(data)
}

// importSettings imports settings from JSON
func (h *Handler) importSettings(w http.ResponseWriter, r *http.Request) {
	var data json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := h.config.ImportSettings(data); err != nil {
		http.Error(w, "Failed to import settings: "+err.Error(), http.StatusBadRequest)
		return
	}

	h.proxyManager.RefreshProxies()

	log.Println("Settings imported successfully")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Settings imported successfully"})
}

// getSettings returns current settings
func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	settings := h.config.GetSettings()
	
	// Add local IP address
	localIP := getLocalIP()
	
	response := map[string]interface{}{
		"file_server_port": settings.FileServerPort,
		"file_server_dir":  settings.FileServerDir,
		"proxy_rules":      settings.ProxyRules,
		"local_ip":         localIP,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// getLocalIP returns the local IP address of the machine
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "Unable to detect"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	
	return "Unable to detect"
}
