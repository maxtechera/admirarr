package keys

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/maxtechera/admirarr/internal/config"
)

var (
	cache   = make(map[string]string)
	cacheMu sync.Mutex

	xmlKeyRe = regexp.MustCompile(`<ApiKey>([^<]+)</ApiKey>`)
	plexRe   = regexp.MustCompile(`PlexOnlineToken="([^"]+)"`)
)

// Get returns the API key for a service, using cache and auto-discovery.
func Get(service string) string {
	if key := config.ManualKey(service); key != "" {
		return key
	}

	cacheMu.Lock()
	if key, ok := cache[service]; ok {
		cacheMu.Unlock()
		return key
	}
	cacheMu.Unlock()

	key := discover(service)
	if key != "" {
		cacheMu.Lock()
		cache[service] = key
		cacheMu.Unlock()
	}
	return key
}

func discover(service string) string {
	switch service {
	case "sonarr":
		return readXMLKey("/mnt/c/ProgramData/Sonarr/config.xml")
	case "radarr":
		return readXMLKey("/mnt/c/ProgramData/Radarr/config.xml")
	case "prowlarr":
		return readXMLKey("/mnt/c/ProgramData/Prowlarr/config.xml")
	case "plex":
		return readPlexToken()
	case "seerr":
		return readSeerrKey()
	case "tautulli":
		return readTautulliKey()
	default:
		return ""
	}
}

func readXMLKey(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	m := xmlKeyRe.FindSubmatch(data)
	if m == nil {
		return ""
	}
	return string(m[1])
}

func readPlexToken() string {
	path := "/mnt/c/Users/Max/AppData/Local/Plex/Plex Media Server/Preferences.xml"
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	m := plexRe.FindSubmatch(data)
	if m == nil {
		return ""
	}
	return string(m[1])
}

func readSeerrKey() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", "exec", "seerr", "cat", "/app/config/settings.json").Output()
	if err != nil {
		return ""
	}
	var result struct {
		Main struct {
			APIKey string `json:"apiKey"`
		} `json:"main"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return ""
	}
	return result.Main.APIKey
}

func readTautulliKey() string {
	paths := []string{
		"/mnt/c/ProgramData/Tautulli/config.ini",
		"/mnt/c/Users/Max/AppData/Local/Tautulli/config.ini",
		"/mnt/c/Users/Max/AppData/Roaming/Tautulli/config.ini",
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "api_key") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
	}
	return ""
}
