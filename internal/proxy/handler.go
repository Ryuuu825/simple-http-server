package proxy

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"simple.http.server/internal/config"
)

// ProxyManager manages dynamic reverse proxies
type ProxyManager struct {
	mu      sync.RWMutex
	proxies map[string]*httputil.ReverseProxy
	config  *config.Config
}

// NewProxyManager creates a new proxy manager
func NewProxyManager(cfg *config.Config) *ProxyManager {
	return &ProxyManager{
		proxies: make(map[string]*httputil.ReverseProxy),
		config:  cfg,
	}
}

// ServeHTTP handles reverse proxy requests
func (pm *ProxyManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rules := pm.config.GetProxyRules()
	
	// Find matching proxy rule
	for _, rule := range rules {
		if strings.HasPrefix(r.URL.Path, rule.PathPrefix) {
			// Get or create proxy for this rule
			proxy := pm.getOrCreateProxy(rule)
			
			if proxy == nil {
				http.Error(w, "Proxy configuration error", http.StatusInternalServerError)
				return
			}
			
			// Modify request path if needed
			originalPath := r.URL.Path
			if rule.StripPrefix {
				r.URL.Path = strings.TrimPrefix(r.URL.Path, rule.PathPrefix)
				if r.URL.Path == "" {
					r.URL.Path = "/"
				}
			}
			
			log.Printf("Proxying %s -> %s%s", originalPath, rule.TargetURL, r.URL.Path)
			
			// Proxy the request
			proxy.ServeHTTP(w, r)
			return
		}
	}
	
	// No matching rule found
	http.Error(w, "No proxy rule matches this path", http.StatusNotFound)
}

// getOrCreateProxy gets an existing proxy or creates a new one
func (pm *ProxyManager) getOrCreateProxy(rule config.ProxyRule) *httputil.ReverseProxy {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	// Check if proxy already exists
	if proxy, exists := pm.proxies[rule.ID]; exists {
		return proxy
	}
	
	// Parse target URL
	targetURL, err := url.Parse(rule.TargetURL)
	if err != nil {
		log.Printf("Error parsing target URL %s: %v", rule.TargetURL, err)
		return nil
	}
	
	// Create new reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	
	// Customize the director to handle headers
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = targetURL.Host
		req.Header.Set("X-Forwarded-Host", req.Host)
		req.Header.Set("X-Forwarded-Proto", "http")
	}
	
	// Custom error handler
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("Proxy error for %s: %v", rule.TargetURL, err)
		http.Error(w, "Proxy error: "+err.Error(), http.StatusBadGateway)
	}
	
	pm.proxies[rule.ID] = proxy
	log.Printf("Created proxy for %s -> %s", rule.PathPrefix, rule.TargetURL)
	
	return proxy
}

// RefreshProxies clears the proxy cache to force recreation with new config
func (pm *ProxyManager) RefreshProxies() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	log.Println("Refreshing all proxies")
	pm.proxies = make(map[string]*httputil.ReverseProxy)
}

// ServePortProxy handles port-based reverse proxy requests
func (pm *ProxyManager) ServePortProxy(w http.ResponseWriter, r *http.Request, rule config.ProxyRule) {
	proxy := pm.getOrCreateProxy(rule)
	
	if proxy == nil {
		http.Error(w, "Proxy configuration error", http.StatusInternalServerError)
		return
	}
	
	log.Printf("Port proxy: localhost:%d%s -> %s%s", rule.Port, r.URL.Path, rule.TargetURL, r.URL.Path)
	
	// Proxy the request
	proxy.ServeHTTP(w, r)
}
