package setup

import (
	"fmt"
	"strings"

	"github.com/maxtechera/admirarr/internal/api"
	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/ui"
)

// SyncQualityProfiles runs Phase 6a: sync quality profiles on all movies/shows.
func SyncQualityProfiles(state *SetupState) StepResult {
	r := StepResult{Name: "Quality Profile Sync"}

	profile := config.QualityProfile()
	if profile == "" {
		fmt.Printf("  %s quality_profile not set in config, skipping\n", ui.Dim("—"))
		r.skip()
		return r
	}

	fmt.Printf("\n  %s\n", ui.Bold("Quality Profile: "+profile))

	// Sync Radarr
	if svc := state.Services["radarr"]; svc != nil && svc.Reachable {
		syncRadarrQuality(profile, &r)
	}

	// Sync Sonarr
	if svc := state.Services["sonarr"]; svc != nil && svc.Reachable {
		syncSonarrQuality(profile, &r)
	}

	return r
}

func syncRadarrQuality(profile string, r *StepResult) {
	// Get quality profiles
	profileID := resolveProfileID("radarr", "api/v3/qualityprofile", profile)
	if profileID == 0 {
		r.errf("Radarr: quality profile %q not found", profile)
		return
	}

	// Get all movies
	var movies []struct {
		ID               int    `json:"id"`
		Title            string `json:"title"`
		QualityProfileID int    `json:"qualityProfileId"`
	}
	if err := api.GetJSON("radarr", "api/v3/movie", nil, &movies); err != nil {
		r.errf("Radarr: cannot fetch movies: %v", err)
		return
	}

	changed := 0
	for _, m := range movies {
		if m.QualityProfileID == profileID {
			continue
		}
		// Fetch full movie, update profile, PUT back
		var full map[string]interface{}
		if err := api.GetJSON("radarr", fmt.Sprintf("api/v3/movie/%d", m.ID), nil, &full); err != nil {
			continue
		}
		full["qualityProfileId"] = profileID
		if _, err := api.Put("radarr", fmt.Sprintf("api/v3/movie/%d", m.ID), full, nil); err != nil {
			r.errf("Radarr: failed to update %s: %v", m.Title, err)
			continue
		}
		changed++
	}

	if changed > 0 {
		fmt.Printf("  %s Radarr: updated %d movies to %s\n", ui.Ok("✓"), changed, profile)
		r.fix()
	} else {
		fmt.Printf("  %s Radarr: all %d movies already on %s\n", ui.Ok("✓"), len(movies), profile)
		r.pass()
	}
}

func syncSonarrQuality(profile string, r *StepResult) {
	profileID := resolveProfileID("sonarr", "api/v3/qualityprofile", profile)
	if profileID == 0 {
		r.errf("Sonarr: quality profile %q not found", profile)
		return
	}

	var series []struct {
		ID               int    `json:"id"`
		Title            string `json:"title"`
		QualityProfileID int    `json:"qualityProfileId"`
	}
	if err := api.GetJSON("sonarr", "api/v3/series", nil, &series); err != nil {
		r.errf("Sonarr: cannot fetch series: %v", err)
		return
	}

	changed := 0
	for _, s := range series {
		if s.QualityProfileID == profileID {
			continue
		}
		var full map[string]interface{}
		if err := api.GetJSON("sonarr", fmt.Sprintf("api/v3/series/%d", s.ID), nil, &full); err != nil {
			continue
		}
		full["qualityProfileId"] = profileID
		if _, err := api.Put("sonarr", fmt.Sprintf("api/v3/series/%d", s.ID), full, nil); err != nil {
			r.errf("Sonarr: failed to update %s: %v", s.Title, err)
			continue
		}
		changed++
	}

	if changed > 0 {
		fmt.Printf("  %s Sonarr: updated %d series to %s\n", ui.Ok("✓"), changed, profile)
		r.fix()
	} else {
		fmt.Printf("  %s Sonarr: all %d series already on %s\n", ui.Ok("✓"), len(series), profile)
		r.pass()
	}
}

func resolveProfileID(service, endpoint, name string) int {
	var profiles []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := api.GetJSON(service, endpoint, nil, &profiles); err != nil {
		return 0
	}
	for _, p := range profiles {
		if strings.EqualFold(p.Name, name) {
			return p.ID
		}
	}
	return 0
}
