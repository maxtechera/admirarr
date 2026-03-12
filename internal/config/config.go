package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// ServiceConfig holds the configuration for a single service.
type ServiceConfig struct {
	Host          string `mapstructure:"host"`
	Port          int    `mapstructure:"port"`
	Type          string `mapstructure:"type"`
	APIVer        string `mapstructure:"api_ver"`
	LocalhostOnly bool   `mapstructure:"localhost_only"`
}

// IndexerConfig holds declarative config for a single Prowlarr indexer.
type IndexerConfig struct {
	Enabled     *bool                  `mapstructure:"enabled"`
	Flare       bool                   `mapstructure:"flare"`
	BaseURL     string                 `mapstructure:"base_url"`
	ExtraFields map[string]interface{} `mapstructure:"extra_fields"`
}

// IsEnabled returns whether the indexer is enabled (defaults to true).
func (ic IndexerConfig) IsEnabled() bool {
	if ic.Enabled == nil {
		return true
	}
	return *ic.Enabled
}

// Config holds the full application configuration.
type Config struct {
	Host           string                   `mapstructure:"host"`
	WSLGateway     string                   `mapstructure:"wsl_gateway"`
	Media          MediaConfig              `mapstructure:"media"`
	Services       map[string]ServiceConfig `mapstructure:"services"`
	Keys           map[string]string        `mapstructure:"keys"`
	QualityProfile string                   `mapstructure:"quality_profile"`
	Indexers       map[string]IndexerConfig `mapstructure:"indexers"`
}

// MediaConfig holds media path configuration.
type MediaConfig struct {
	WSL string `mapstructure:"wsl"`
	Win string `mapstructure:"win"`
}

// Default service definitions matching the Python CLI.
var defaultServices = map[string]ServiceConfig{
	"plex":         {Port: 32400, Type: "windows"},
	"qbittorrent":  {Port: 8080, Type: "windows"},
	"prowlarr":     {Port: 9696, Type: "windows", APIVer: "v1"},
	"sonarr":       {Port: 8989, Type: "windows", APIVer: "v3"},
	"radarr":       {Port: 7878, Type: "windows", APIVer: "v3"},
	"tautulli":     {Port: 8181, Type: "windows"},
	"seerr":        {Host: "localhost", Port: 5055, Type: "docker"},
	"bazarr":       {Host: "localhost", Port: 6767, Type: "docker"},
	"organizr":     {Host: "localhost", Port: 9983, Type: "docker"},
	"flaresolverr": {Host: "localhost", Port: 8191, Type: "docker"},
}

var cfg *Config

// Load initializes the configuration from file and defaults.
func Load() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	configDir := filepath.Join(os.Getenv("HOME"), ".config", "admirarr")
	viper.AddConfigPath(configDir)
	viper.AddConfigPath(".")

	// Set defaults
	viper.SetDefault("host", "192.168.50.42")
	viper.SetDefault("media.wsl", "/mnt/d/Media")
	viper.SetDefault("media.win", `D:\Media`)

	_ = viper.ReadInConfig() // OK if missing

	cfg = &Config{
		Host:           viper.GetString("host"),
		WSLGateway:     viper.GetString("wsl_gateway"),
		Media: MediaConfig{
			WSL: viper.GetString("media.wsl"),
			Win: viper.GetString("media.win"),
		},
		Services:       make(map[string]ServiceConfig),
		Keys:           make(map[string]string),
		QualityProfile: viper.GetString("quality_profile"),
		Indexers:       make(map[string]IndexerConfig),
	}

	// Resolve WSL gateway
	if cfg.WSLGateway == "" || cfg.WSLGateway == "auto" {
		cfg.WSLGateway = detectWSLGateway()
	}

	// Load services from config or use defaults
	if viper.IsSet("services") {
		svcMap := viper.GetStringMap("services")
		for name := range svcMap {
			var svc ServiceConfig
			sub := viper.Sub("services." + name)
			if sub != nil {
				_ = sub.Unmarshal(&svc)
			}
			// Fill host from global default if empty
			if svc.Host == "" {
				if def, ok := defaultServices[name]; ok && def.Host != "" {
					svc.Host = def.Host
				} else {
					svc.Host = cfg.Host
				}
			}
			// Fill api_ver from defaults if empty
			if svc.APIVer == "" {
				if def, ok := defaultServices[name]; ok {
					svc.APIVer = def.APIVer
				}
			}
			cfg.Services[name] = svc
		}
	}

	// Ensure all default services exist
	for name, def := range defaultServices {
		if _, exists := cfg.Services[name]; !exists {
			svc := def
			if svc.Host == "" {
				svc.Host = cfg.Host
			}
			cfg.Services[name] = svc
		}
	}

	// Load manual keys
	if viper.IsSet("keys") {
		keyMap := viper.GetStringMapString("keys")
		for k, v := range keyMap {
			if v != "" {
				cfg.Keys[k] = v
			}
		}
	}

	// Load indexer config
	if viper.IsSet("indexers") {
		idxMap := viper.GetStringMap("indexers")
		for name := range idxMap {
			var ic IndexerConfig
			sub := viper.Sub("indexers." + name)
			if sub != nil {
				_ = sub.Unmarshal(&ic)
			}
			cfg.Indexers[name] = ic
		}
	}

	// Apply localhost_only routing: services that bind to localhost on Windows
	// need to be accessed via the WSL gateway IP instead of the host IP.
	for name, svc := range cfg.Services {
		if svc.LocalhostOnly && cfg.WSLGateway != "" {
			svc.Host = cfg.WSLGateway
			cfg.Services[name] = svc
		}
	}
}

// Get returns the current configuration.
func Get() *Config {
	if cfg == nil {
		Load()
	}
	return cfg
}

// ServiceURL returns the base URL for a service.
func ServiceURL(name string) string {
	svc := Get().Services[name]
	return fmt.Sprintf("http://%s:%d", svc.Host, svc.Port)
}

// ServiceHost returns the host for a service.
func ServiceHost(name string) string {
	return Get().Services[name].Host
}

// ServicePort returns the port for a service.
func ServicePort(name string) int {
	return Get().Services[name].Port
}

// ServiceType returns the type (windows/docker) for a service.
func ServiceType(name string) string {
	return Get().Services[name].Type
}

// ServiceAPIVer returns the API version for a service.
func ServiceAPIVer(name string) string {
	return Get().Services[name].APIVer
}

// MediaPathWSL returns the WSL media path.
func MediaPathWSL() string {
	return Get().Media.WSL
}

// MediaPathWin returns the Windows media path.
func MediaPathWin() string {
	return Get().Media.Win
}

// Host returns the configured host.
func Host() string {
	return Get().Host
}

// ManualKey returns a manually configured API key, or empty string.
func ManualKey(service string) string {
	return Get().Keys[service]
}

// AllServiceNames returns all service names in a consistent order.
func AllServiceNames() []string {
	return []string{
		"plex", "qbittorrent", "prowlarr", "sonarr", "radarr",
		"tautulli", "seerr", "bazarr", "organizr", "flaresolverr",
	}
}

// DockerServices returns the names of Docker-based services.
func DockerServices() []string {
	return []string{"seerr", "bazarr", "organizr", "flaresolverr"}
}

// QualityProfile returns the configured quality profile name.
func QualityProfile() string {
	return Get().QualityProfile
}

// GetIndexers returns the declarative indexer config.
func GetIndexers() map[string]IndexerConfig {
	return Get().Indexers
}

// WSLGateway returns the resolved WSL gateway IP.
func WSLGateway() string {
	return Get().WSLGateway
}

// detectWSLGateway reads the default gateway from /etc/resolv.conf (WSL2)
// or falls back to ip route parsing.
func detectWSLGateway() string {
	// Try /etc/resolv.conf first (most reliable on WSL2)
	data, err := os.ReadFile("/etc/resolv.conf")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "nameserver") {
				parts := strings.Fields(line)
				if len(parts) >= 2 && parts[1] != "127.0.0.1" {
					return parts[1]
				}
			}
		}
	}
	return ""
}
