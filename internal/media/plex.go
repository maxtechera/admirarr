package media

import (
	"encoding/json"
	"fmt"

	"github.com/maxtechera/admirarr/internal/api"
	"github.com/maxtechera/admirarr/internal/keys"
)

// PlexServer implements MediaServer for Plex + Tautulli.
type PlexServer struct{}

// Name returns the display name.
func (p *PlexServer) Name() string { return "Plex" }

// RecentlyAdded returns the most recently added items from Plex.
// addAuth sets Accept: application/json for Plex, so we parse JSON.
func (p *PlexServer) RecentlyAdded(limit int) ([]MediaItem, error) {
	body, err := api.Get("plex", "library/recentlyAdded", nil)
	if err != nil {
		return nil, err
	}

	var container struct {
		MediaContainer struct {
			Metadata []struct {
				Title            string `json:"title"`
				Year             int    `json:"year"`
				Type             string `json:"type"`
				GrandparentTitle string `json:"grandparentTitle"`
			} `json:"Metadata"`
		} `json:"MediaContainer"`
	}
	if err := json.Unmarshal(body, &container); err != nil {
		return nil, fmt.Errorf("parse Plex response: %w", err)
	}

	var result []MediaItem
	for i, v := range container.MediaContainer.Metadata {
		if i >= limit {
			break
		}
		label := v.Type
		name := v.Title
		if v.Type == "episode" && v.GrandparentTitle != "" {
			name = v.GrandparentTitle + " — " + v.Title
			label = "Episode"
		} else if v.Type == "movie" {
			label = "Movie"
		}
		year := "?"
		if v.Year > 0 {
			year = fmt.Sprintf("%d", v.Year)
		}
		result = append(result, MediaItem{Title: name, Year: year, Type: label})
	}
	return result, nil
}

// LibraryScan triggers a scan on all Plex library sections.
func (p *PlexServer) LibraryScan() ([]ScanResult, error) {
	var sections struct {
		MediaContainer struct {
			Directory []struct {
				Key   string `json:"key"`
				Title string `json:"title"`
			} `json:"Directory"`
		} `json:"MediaContainer"`
	}
	if err := api.GetJSON("plex", "library/sections", nil, &sections); err != nil {
		return nil, fmt.Errorf("get Plex sections: %w", err)
	}

	var results []ScanResult
	for _, dir := range sections.MediaContainer.Directory {
		endpoint := fmt.Sprintf("library/sections/%s/refresh", dir.Key)
		_, err := api.Post("plex", endpoint, nil, nil)
		if err != nil {
			results = append(results, ScanResult{Library: dir.Title, OK: false, Err: err})
		} else {
			results = append(results, ScanResult{Library: dir.Title, OK: true})
		}
	}
	return results, nil
}

// WatchHistory returns watch history from Tautulli (required for Plex history).
func (p *PlexServer) WatchHistory(limit int) ([]WatchEntry, error) {
	key := keys.Get("tautulli")
	if key == "" || !api.CheckReachable("tautulli") {
		return nil, fmt.Errorf("tautulli is required for Plex watch history but is not available")
	}
	return tautulliHistory(key, limit)
}
