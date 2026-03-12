package setup

import (
	"fmt"
	"net/http"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/keys"
	"github.com/maxtechera/admirarr/internal/ui"
)

// keyed services that require API key validation.
var keyedServices = []string{"sonarr", "radarr", "prowlarr", "plex", "tautulli", "seerr"}

// ValidateAPIKeys runs Phase 3: API key discovery and validation.
func ValidateAPIKeys(state *SetupState) StepResult {
	r := StepResult{Name: "API Key Validation"}

	fmt.Println(ui.Bold("\n  Phase 3 — API Key Validation"))
	fmt.Println(ui.Separator())

	for _, name := range keyedServices {
		svc := state.Services[name]
		if !svc.Reachable {
			fmt.Printf("  %s %-15s %s\n", ui.Dim("—"), name, ui.Dim("skipped (not reachable)"))
			r.skip()
			continue
		}

		// Try auto-discovery
		key := keys.Get(name)
		if key != "" && validateKey(name, key, svc) {
			svc.APIKey = key
			state.Keys[name] = key
			masked := maskKey(key)
			fmt.Printf("  %s %-15s %s %s\n", ui.Ok("✓"), name, ui.Ok("valid"), ui.Dim(masked))
			r.pass()
			continue
		}

		if key != "" {
			fmt.Printf("  %s %-15s %s\n", ui.Warn("!"), name, ui.Warn("key found but validation failed"))
		} else {
			fmt.Printf("  %s %-15s %s\n", ui.Err("✗"), name, ui.Err("key not found"))
		}

		// Prompt for manual entry
		var manualKey string
		port := config.ServicePort(name)
		host := config.ServiceHost(name)
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title(fmt.Sprintf("Enter API key for %s (Settings > General at http://%s:%d)", name, host, port)).
					Value(&manualKey),
			),
		)
		if err := form.Run(); err != nil || manualKey == "" {
			r.errf("%s: no API key available", name)
			continue
		}

		if validateKey(name, manualKey, svc) {
			svc.APIKey = manualKey
			state.Keys[name] = manualKey
			state.ManualKeys[name] = manualKey
			fmt.Printf("  %s %-15s %s\n", ui.Ok("✓"), name, ui.GoldText("key accepted"))
			r.fix()
		} else {
			r.errf("%s: entered key failed validation", name)
		}
	}

	return r
}

func validateKey(service, key string, svc *ServiceState) bool {
	c := &http.Client{Timeout: 5 * time.Second}

	var url string
	switch service {
	case "sonarr", "radarr":
		ver := config.ServiceAPIVer(service)
		url = fmt.Sprintf("http://%s:%d/api/%s/system/status?apikey=%s", svc.Host, svc.Port, ver, key)
	case "prowlarr":
		url = fmt.Sprintf("http://%s:%d/api/v1/system/status?apikey=%s", svc.Host, svc.Port, key)
	case "plex":
		url = fmt.Sprintf("http://%s:%d/identity?X-Plex-Token=%s", svc.Host, svc.Port, key)
	case "tautulli":
		url = fmt.Sprintf("http://%s:%d/api/v2?apikey=%s&cmd=status", svc.Host, svc.Port, key)
	case "seerr":
		req, err := http.NewRequest("GET", fmt.Sprintf("http://%s:%d/api/v1/status", svc.Host, svc.Port), nil)
		if err != nil {
			return false
		}
		req.Header.Set("X-Api-Key", key)
		resp, err := c.Do(req)
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == 200
	default:
		return key != ""
	}

	resp, err := c.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "…" + key[len(key)-4:]
}
