package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"simple.http.server/internal/admin"
	"simple.http.server/internal/config"
	"simple.http.server/internal/fileserver"
	"simple.http.server/internal/proxy"
)

func main() {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	// Initialize configuration
	cfg := config.GetConfig()
	cfg.SetFileServerDir(cwd)

	// Initialize components
	fileServer := fileserver.NewFileServer(cfg)
	proxyManager := proxy.NewProxyManager(cfg)
	adminHandler := admin.NewHandler(cfg, proxyManager)

	// Setup routes
	mux := http.NewServeMux()

	// Admin panel routes
	mux.Handle("/admin/api/", adminHandler)
	mux.Handle("/admin/", http.StripPrefix("/admin", admin.GetStaticHandler()))

	// SSE endpoint for file changes
	mux.HandleFunc("/events", fileServer.HandleSSE)

	// Main router to handle proxy vs file server
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Check if this path matches any proxy rule
		rules := cfg.GetProxyRules()
		for _, rule := range rules {
			if rule.PathPrefix != "" && strings.HasPrefix(r.URL.Path, rule.PathPrefix) {
				proxyManager.ServeHTTP(w, r)
				return
			}
		}

		// No proxy match, serve files
		fileServer.ServeHTTP(w, r)
	})

	// Start port-based proxies
	go startPortBasedProxies(cfg, proxyManager)

	// Find an available port (use 0 to let OS assign one)
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatalf("Failed to find available port: %v", err)
	}
	
	// Get the actual port assigned
	port := listener.Addr().(*net.TCPAddr).Port
	
	// Update config with the actual port
	cfg.SetFileServerPort(port)

	// Print startup information
	log.Println("╔════════════════════════════════════════════════════════════╗")
	log.Println("║          Simple HTTP Server - 2 in 1                       ║")
	log.Println("╚════════════════════════════════════════════════════════════╝")
	log.Printf("📁 File Server:    http://localhost:%d/", port)
	log.Printf("📂 Serving from:   %s", cwd)
	log.Printf("⚙️  Admin Panel:    http://localhost:%d/admin/", port)
	log.Printf("🔄 Live Updates:   Enabled (SSE)")
	log.Println("────────────────────────────────────────────────────────────")
	log.Printf("Server starting on :%d", port)
	log.Println("Press Ctrl+C to stop")
	log.Println("")

	// Open admin panel in browser
	adminURL := fmt.Sprintf("http://localhost:%d/admin/", port)
	go openBrowser(adminURL)

	// Start server with the listener we already created
	if err := http.Serve(listener, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// openBrowser opens the specified URL in the default browser
func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Printf("Failed to open browser: %v", err)
	}
}

// startPortBasedProxies starts separate servers for port-based proxy rules
func startPortBasedProxies(cfg *config.Config, proxyManager *proxy.ProxyManager) {
	rules := cfg.GetProxyRules()
	for _, rule := range rules {
		if rule.Port > 0 {
			go func(r config.ProxyRule) {
				addr := fmt.Sprintf(":%d", r.Port)
				log.Printf("🔗 Port Proxy:     http://localhost:%d -> %s", r.Port, r.TargetURL)
				
				handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					proxyManager.ServePortProxy(w, req, r)
				})
				
				if err := http.ListenAndServe(addr, handler); err != nil {
					log.Printf("Port-based proxy failed on port %d: %v", r.Port, err)
				}
			}(rule)
		}
	}
}
