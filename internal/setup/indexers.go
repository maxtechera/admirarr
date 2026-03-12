package setup

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/maxtechera/admirarr/internal/api"
	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/ui"
)

// indexerDef maps config names to Prowlarr API parameters.
var indexerRegistry = map[string]struct {
	Implementation string
	ConfigContract string
	DefinitionFile string
}{
	"1337x":           {"Cardigann", "CardigannSettings", "1337x"},
	"The Pirate Bay":  {"Cardigann", "CardigannSettings", "thepiratebay"},
	"TorrentGalaxy":   {"Cardigann", "CardigannSettings", "torrentgalaxyclone"},
	"Knaben":          {"Knaben", "NoAuthTorrentBaseSettings", ""},
	"YTS":             {"Cardigann", "CardigannSettings", "yts"},
	"EZTV":            {"Cardigann", "CardigannSettings", "eztv"},
	"Nyaa.si":         {"Cardigann", "CardigannSettings", "nyaasi"},
	"SubsPlease":      {"SubsPlease", "SubsPleaseSettings", ""},
	"Anidex":          {"Anidex", "AnidexSettings", ""},
	"Tokyo Toshokan":  {"Cardigann", "CardigannSettings", "tokyotosho"},
}

type prowlarrIndexer struct {
	ID             int                      `json:"id"`
	Name           string                   `json:"name"`
	Enable         bool                     `json:"enable"`
	Tags           []int                    `json:"tags"`
	Fields         []map[string]interface{} `json:"fields,omitempty"`
	Implementation string                   `json:"implementation"`
	ConfigContract string                   `json:"configContract"`
}

// VerifyIndexers runs Phase 6: declarative indexer sync.
// Reads config.Indexers and converges Prowlarr to match.
func VerifyIndexers(state *SetupState) StepResult {
	r := StepResult{Name: "Indexer Sync"}

	fmt.Println(ui.Bold("\n  Phase 6 — Indexer Sync"))
	fmt.Println(ui.Separator())

	svc := state.Services["prowlarr"]
	if svc == nil || !svc.Reachable {
		fmt.Printf("  %s Prowlarr %s\n", ui.Dim("—"), ui.Dim("skipped (not reachable)"))
		r.skip()
		return r
	}

	desired := config.GetIndexers()
	if len(desired) == 0 {
		fmt.Printf("  %s No indexers declared in config, skipping\n", ui.Dim("—"))
		r.skip()
		// Still check sync targets
		checkSyncTargets(state, &r)
		return r
	}

	// Get current indexers from Prowlarr
	var existing []prowlarrIndexer
	if err := api.GetJSON("prowlarr", "api/v1/indexer", nil, &existing); err != nil {
		r.errf("cannot query Prowlarr indexers: %v", err)
		return r
	}

	existingByName := make(map[string]prowlarrIndexer)
	for _, idx := range existing {
		existingByName[strings.ToLower(idx.Name)] = idx
	}

	// Get FlareSolverr tag
	flareTag := getFlareTag()
	if flareTag > 0 {
		fmt.Printf("  %s FlareSolverr detected (tag %d)\n", ui.Ok("●"), flareTag)
	}

	// Converge: add missing, update flare tags
	for name, ic := range desired {
		if !ic.IsEnabled() {
			continue
		}

		lower := strings.ToLower(name)
		if idx, ok := existingByName[lower]; ok {
			// Already exists — check flare tag
			updated := ensureFlareTag(idx, ic.Flare, flareTag)
			if updated {
				fmt.Printf("  %s %s — updated flare tag\n", ui.GoldText("↻"), name)
				r.fix()
			} else {
				fmt.Printf("  %s %s\n", ui.Ok("✓"), name)
				r.pass()
			}
			delete(existingByName, lower)
		} else {
			// Need to add
			added := addIndexer(name, ic, flareTag)
			if added {
				fmt.Printf("  %s %s — added\n", ui.Ok("+"), name)
				r.fix()
			} else {
				r.errf("failed to add indexer: %s", name)
			}
		}
	}

	// Remove indexers not in config
	for _, idx := range existingByName {
		// Only remove if it matches a known indexer from the registry
		// (don't touch manually-added custom indexers)
		if _, known := indexerRegistry[idx.Name]; known {
			if _, err := api.Delete("prowlarr", fmt.Sprintf("api/v1/indexer/%d", idx.ID), nil); err != nil {
				r.errf("failed to remove %s: %v", idx.Name, err)
			} else {
				fmt.Printf("  %s %s — removed (not in config)\n", ui.Dim("−"), idx.Name)
				r.fix()
			}
		}
	}

	// Check sync targets
	checkSyncTargets(state, &r)

	return r
}

// SyncIndexers is the standalone version called by `indexers sync`.
func SyncIndexers() StepResult {
	state := &SetupState{
		Services: make(map[string]*ServiceState),
	}
	state.Services["prowlarr"] = &ServiceState{Reachable: api.CheckReachable("prowlarr")}
	state.Services["radarr"] = &ServiceState{Reachable: api.CheckReachable("radarr")}
	state.Services["sonarr"] = &ServiceState{Reachable: api.CheckReachable("sonarr")}
	return VerifyIndexers(state)
}

func getFlareTag() int {
	var proxies []struct {
		Tags []int `json:"tags"`
	}
	if api.GetJSON("prowlarr", "api/v1/indexerProxy", nil, &proxies) == nil {
		for _, p := range proxies {
			if len(p.Tags) > 0 {
				return p.Tags[0]
			}
		}
	}
	return 0
}

func ensureFlareTag(idx prowlarrIndexer, wantFlare bool, flareTag int) bool {
	if flareTag == 0 || !wantFlare {
		return false
	}

	hasTag := false
	for _, t := range idx.Tags {
		if t == flareTag {
			hasTag = true
			break
		}
	}

	if hasTag == wantFlare {
		return false
	}

	// Need to update
	tags := idx.Tags
	if wantFlare && !hasTag {
		tags = append(tags, flareTag)
	}

	payload := map[string]interface{}{
		"id":   idx.ID,
		"tags": tags,
	}

	// GET full indexer first to have complete payload
	var full map[string]interface{}
	if err := api.GetJSON("prowlarr", fmt.Sprintf("api/v1/indexer/%d", idx.ID), nil, &full); err != nil {
		return false
	}
	full["tags"] = tags

	_, err := api.Put("prowlarr", fmt.Sprintf("api/v1/indexer/%d", idx.ID), full, nil)
	_ = payload // used for documentation
	return err == nil
}

func addIndexer(name string, ic config.IndexerConfig, flareTag int) bool {
	reg, ok := indexerRegistry[name]
	if !ok {
		// Try case-insensitive match
		for rName, rDef := range indexerRegistry {
			if strings.EqualFold(rName, name) {
				reg = rDef
				name = rName // use canonical name
				ok = true
				break
			}
		}
		if !ok {
			fmt.Printf("  %s %s — unknown indexer, skipping\n", ui.Warn("?"), name)
			return false
		}
	}

	fields := []map[string]interface{}{}

	if reg.DefinitionFile != "" {
		fields = append(fields, map[string]interface{}{
			"name": "definitionFile", "value": reg.DefinitionFile,
		})
	}

	if ic.BaseURL != "" {
		fields = append(fields, map[string]interface{}{
			"name": "baseUrl", "value": ic.BaseURL,
		})
	}

	for k, v := range ic.ExtraFields {
		fields = append(fields, map[string]interface{}{
			"name": k, "value": v,
		})
	}

	tags := []int{}
	if ic.Flare && flareTag > 0 {
		tags = append(tags, flareTag)
	}

	// Add disabled first to bypass connectivity validation
	payload := map[string]interface{}{
		"enable":         false,
		"name":           name,
		"implementation": reg.Implementation,
		"configContract": reg.ConfigContract,
		"appProfileId":   1,
		"protocol":       "torrent",
		"priority":       25,
		"tags":           tags,
		"fields":         fields,
	}

	body, err := api.Post("prowlarr", "api/v1/indexer", payload, nil)
	if err != nil {
		fmt.Printf("  %s %s — POST failed: %v\n", ui.Err("✗"), name, err)
		return false
	}

	var created struct {
		ID int `json:"id"`
	}
	if json.Unmarshal(body, &created) != nil || created.ID == 0 {
		// Check for validation error
		var errs []struct {
			ErrorMessage string `json:"errorMessage"`
		}
		if json.Unmarshal(body, &errs) == nil && len(errs) > 0 {
			fmt.Printf("  %s %s — %s\n", ui.Err("✗"), name, errs[0].ErrorMessage)
		}
		return false
	}

	// Now enable via PUT
	payload["enable"] = true
	payload["id"] = created.ID
	_, err = api.Put("prowlarr", fmt.Sprintf("api/v1/indexer/%d", created.ID), payload, nil)
	if err != nil {
		flareNote := ""
		if ic.Flare {
			flareNote = " (needs FlareSolverr)"
		}
		fmt.Printf("  %s %s — added disabled%s\n", ui.Warn("⚠"), name, flareNote)
		return true // still added, just disabled
	}

	return true
}

func checkSyncTargets(state *SetupState, r *StepResult) {
	var apps []struct {
		ID             int    `json:"id"`
		Name           string `json:"name"`
		Implementation string `json:"implementation"`
		SyncLevel      string `json:"syncLevel"`
	}
	if err := api.GetJSON("prowlarr", "api/v1/applications", nil, &apps); err != nil {
		return
	}

	fmt.Printf("\n  %s\n", ui.Bold("Sync Targets"))

	radarrSynced, sonarrSynced := false, false
	for _, app := range apps {
		name := strings.ToLower(app.Name)
		if strings.Contains(name, "radarr") || app.Implementation == "Radarr" {
			radarrSynced = true
		}
		if strings.Contains(name, "sonarr") || app.Implementation == "Sonarr" {
			sonarrSynced = true
		}
		fmt.Printf("  %s %s (%s, sync: %s)\n", ui.Ok("✓"), app.Name,
			ui.Dim(app.Implementation), ui.Dim(app.SyncLevel))
	}

	if state.Services["radarr"] != nil && state.Services["radarr"].Reachable && !radarrSynced {
		fmt.Printf("  %s Radarr not configured as Prowlarr sync target\n", ui.Warn("!"))
		r.errf("Radarr not synced with Prowlarr — add it in Prowlarr > Settings > Apps")
	}
	if state.Services["sonarr"] != nil && state.Services["sonarr"].Reachable && !sonarrSynced {
		fmt.Printf("  %s Sonarr not configured as Prowlarr sync target\n", ui.Warn("!"))
		r.errf("Sonarr not synced with Prowlarr — add it in Prowlarr > Settings > Apps")
	}

	if (radarrSynced || state.Services["radarr"] == nil) && (sonarrSynced || state.Services["sonarr"] == nil) {
		r.pass()
	}
}
