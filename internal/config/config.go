package config

import (
	"encoding/json"
	"sync"
)

// ProxyRule represents a reverse proxy configuration
type ProxyRule struct {
	ID          string `json:"id"`
	PathPrefix  string `json:"path_prefix"`  // e.g., "/api" (optional if Port is set)
	Port        int    `json:"port"`         // e.g., 8081 (optional, enables port-based proxying)
	TargetURL   string `json:"target_url"`   // e.g., "http://localhost:3000"
	StripPrefix bool   `json:"strip_prefix"` // whether to strip the path prefix when proxying
}

// Settings represents the application configuration
type Settings struct {
	ProxyRules     []ProxyRule `json:"proxy_rules"`
	FileServerPort int         `json:"file_server_port"`
	FileServerDir  string      `json:"file_server_dir"`
}

// Config manages the runtime configuration
type Config struct {
	mu       sync.RWMutex
	settings Settings
}

var globalConfig = &Config{
	settings: Settings{
		ProxyRules:     []ProxyRule{},
		FileServerPort: 8080,
		FileServerDir:  ".",
	},
}

// GetConfig returns the global configuration instance
func GetConfig() *Config {
	return globalConfig
}

// GetSettings returns a copy of current settings
func (c *Config) GetSettings() Settings {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	// Deep copy proxy rules
	rules := make([]ProxyRule, len(c.settings.ProxyRules))
	copy(rules, c.settings.ProxyRules)
	
	return Settings{
		ProxyRules:     rules,
		FileServerPort: c.settings.FileServerPort,
		FileServerDir:  c.settings.FileServerDir,
	}
}

// GetProxyRules returns all proxy rules
func (c *Config) GetProxyRules() []ProxyRule {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	rules := make([]ProxyRule, len(c.settings.ProxyRules))
	copy(rules, c.settings.ProxyRules)
	return rules
}

// AddProxyRule adds a new proxy rule
func (c *Config) AddProxyRule(rule ProxyRule) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.settings.ProxyRules = append(c.settings.ProxyRules, rule)
}

// UpdateProxyRule updates an existing proxy rule
func (c *Config) UpdateProxyRule(id string, rule ProxyRule) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	for i, r := range c.settings.ProxyRules {
		if r.ID == id {
			rule.ID = id // Ensure ID doesn't change
			c.settings.ProxyRules[i] = rule
			return true
		}
	}
	return false
}

// DeleteProxyRule removes a proxy rule by ID
func (c *Config) DeleteProxyRule(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	for i, r := range c.settings.ProxyRules {
		if r.ID == id {
			c.settings.ProxyRules = append(c.settings.ProxyRules[:i], c.settings.ProxyRules[i+1:]...)
			return true
		}
	}
	return false
}

// ExportSettings exports the current settings as JSON
func (c *Config) ExportSettings() ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return json.MarshalIndent(c.settings, "", "  ")
}

// ImportSettings imports settings from JSON
func (c *Config) ImportSettings(data []byte) error {
	var newSettings Settings
	if err := json.Unmarshal(data, &newSettings); err != nil {
		return err
	}
	
	c.mu.Lock()
	defer c.mu.Unlock()
	c.settings = newSettings
	return nil
}

// SetFileServerDir sets the file server directory
func (c *Config) SetFileServerDir(dir string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.settings.FileServerDir = dir
}

// GetFileServerDir gets the file server directory
func (c *Config) GetFileServerDir() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.settings.FileServerDir
}

// SetFileServerPort sets the file server port
func (c *Config) SetFileServerPort(port int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.settings.FileServerPort = port
}

// GetFileServerPort gets the file server port
func (c *Config) GetFileServerPort() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.settings.FileServerPort
}
