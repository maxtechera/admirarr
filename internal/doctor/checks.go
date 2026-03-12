package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/maxtechera/admirarr/internal/api"
	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/keys"
	"github.com/maxtechera/admirarr/internal/ui"
)

// Issue represents a diagnostic issue found.
type Issue struct {
	Description string
}

// Result holds the diagnostic results.
type Result struct {
	Issues       []Issue
	ChecksPassed int
}

// RunChecks runs all 9 diagnostic categories and returns results.
func RunChecks() *Result {
	r := &Result{}

	checkConnectivity(r)
	checkAPIKeys(r)
	checkConfigFiles(r)
	checkDockerContainers(r)
	checkDiskSpace(r)
	checkMediaPaths(r)
	checkRootFolders(r)
	checkIndexers(r)
	checkServiceWarnings(r)

	return r
}

func checkConnectivity(r *Result) {
	fmt.Println(ui.Bold("  Service Connectivity"))
	fmt.Println(ui.Separator())

	// Detect Windows processes
	winProcesses := make(map[string]string)
	out, err := exec.Command("/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe",
		"-Command", "Get-Process | Select-Object Name,Id | Format-Table -HideTableHeaders").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			parts := strings.Fields(strings.TrimSpace(line))
			if len(parts) >= 2 {
				winProcesses[strings.ToLower(parts[0])] = parts[1]
			}
		}
	}

	procNames := map[string]string{
		"sonarr": "sonarr", "radarr": "radarr", "prowlarr": "prowlarr",
		"plex": "plex media server", "tautulli": "tautulli", "qbittorrent": "qbittorrent",
	}
	winServiceNames := map[string]string{
		"sonarr": "Sonarr", "radarr": "Radarr", "prowlarr": "Prowlarr",
		"plex": "Plex Media Server",
	}
	desktopApps := map[string]bool{"qbittorrent": true, "tautulli": true}

	for _, name := range config.AllServiceNames() {
		svc := config.Get().Services[name]
		t0 := time.Now()
		up := api.CheckReachable(name)
		elapsed := time.Since(t0).Milliseconds()

		if up {
			r.ChecksPassed++
			speed := ui.Ok(fmt.Sprintf("%dms", elapsed))
			if elapsed > 2000 {
				speed = ui.Warn(fmt.Sprintf("%dms (slow)", elapsed))
				r.Issues = append(r.Issues, Issue{
					fmt.Sprintf("SLOW SERVICE: %s responded in %dms (>2s). Type: %s. URL: http://%s:%d/. Fix: check %s resource usage or network latency.",
						name, elapsed, svc.Type, svc.Host, svc.Port, name),
				})
			}
			fmt.Printf("  %s %-15s %-12s %s\n", ui.Ok("✓"), name, ui.Dim(fmt.Sprintf(":%d", svc.Port)), speed)
		} else {
			diag := ""
			fixHint := ""
			if svc.Type == "docker" {
				cOut, cErr := exec.Command("docker", "inspect", "-f", "{{.State.Status}}", name).Output()
				cstate := strings.TrimSpace(string(cOut))
				if cErr == nil && cstate == "running" {
					diag = fmt.Sprintf("Container is running but port %d not responding. ", svc.Port)
					fixHint = fmt.Sprintf("Fix: container '%s' is running but not serving on port %d. Check logs: docker logs --tail 30 %s. Try restart: docker restart %s",
						name, svc.Port, name, name)
				} else if cErr == nil && cstate != "" {
					diag = fmt.Sprintf("Container state: %s. ", cstate)
					fixHint = fmt.Sprintf("Fix: run 'docker start %s'. Check logs: docker logs --tail 30 %s", name, name)
				} else {
					fixHint = fmt.Sprintf("Fix: container not found. Run 'docker ps -a | grep %s'", name)
				}
			} else {
				procName := procNames[name]
				procRunning := false
				procPID := ""
				for k, v := range winProcesses {
					if strings.Contains(k, procName) {
						procRunning = true
						procPID = v
						break
					}
				}

				if procRunning {
					diag = fmt.Sprintf("Process IS running (PID %s) but port %d not reachable from WSL. ", procPID, svc.Port)
					if desktopApps[name] {
						fixHint = fmt.Sprintf("Fix: %s process is running (PID %s) but http://%s:%d/ is not responding. This is a desktop app, NOT a Windows service. Check Web UI settings, port binding, or Windows Firewall.",
							name, procPID, svc.Host, svc.Port)
					} else {
						winSvc := winServiceNames[name]
						fixHint = fmt.Sprintf("Fix: %s process is running (PID %s) but http://%s:%d/ is not responding. Try: powershell Restart-Service '%s' -Force. Or check Windows Firewall.",
							name, procPID, svc.Host, svc.Port, winSvc)
					}
				} else {
					diag = "Process NOT found running on Windows. "
					if desktopApps[name] {
						fixHint = fmt.Sprintf("Fix: %s is not running. It is a desktop app — start it from Windows Start Menu.", name)
					} else {
						winSvc := winServiceNames[name]
						fixHint = fmt.Sprintf("Fix: %s is not running. Start: powershell Start-Service '%s'. If that fails: powershell Get-Service '%s'.",
							name, winSvc, winSvc)
					}
				}
			}

			r.Issues = append(r.Issues, Issue{
				fmt.Sprintf("UNREACHABLE: %s (%s) at http://%s:%d/ — %s%s", name, svc.Type, svc.Host, svc.Port, diag, fixHint),
			})
			statusExtra := ""
			if diag != "" {
				short := "not running"
				if strings.Contains(diag, "IS running") {
					short = "process up, port blocked"
				}
				statusExtra = " " + ui.Dim("("+short+")")
			}
			fmt.Printf("  %s %-15s %-12s %s %s%s\n", ui.Err("✗"), name, ui.Dim(fmt.Sprintf(":%d", svc.Port)), ui.Err("unreachable"), ui.Dim("["+svc.Type+"]"), statusExtra)
		}
	}
}

func checkAPIKeys(r *Result) {
	fmt.Println(ui.Bold("\n  API Keys"))
	fmt.Println(ui.Separator())

	keyConfigs := map[string]string{
		"sonarr":   "/mnt/c/ProgramData/Sonarr/config.xml",
		"radarr":   "/mnt/c/ProgramData/Radarr/config.xml",
		"prowlarr": "/mnt/c/ProgramData/Prowlarr/config.xml",
		"plex":     "/mnt/c/Users/Max/AppData/Local/Plex/Plex Media Server/Preferences.xml",
		"tautulli": "/mnt/c/ProgramData/Tautulli/config.ini",
		"seerr":    "docker exec seerr cat /app/config/settings.json",
	}
	altPaths := map[string][]string{
		"tautulli": {
			"/mnt/c/Users/Max/AppData/Local/Tautulli/config.ini",
			"/mnt/c/Users/Max/AppData/Roaming/Tautulli/config.ini",
		},
		"sonarr": {"/mnt/c/Users/Max/AppData/Roaming/Sonarr/config.xml"},
		"radarr": {"/mnt/c/Users/Max/AppData/Roaming/Radarr/config.xml"},
	}

	for _, svc := range []string{"sonarr", "radarr", "prowlarr", "plex", "tautulli", "seerr"} {
		key := keys.Get(svc)
		if key != "" {
			r.ChecksPassed++
			masked := key[:4] + "…" + key[len(key)-4:]
			if len(key) <= 8 {
				masked = "****"
			}
			fmt.Printf("  %s %-15s %s\n", ui.Ok("✓"), svc, ui.Dim(masked))
		} else {
			configPath := keyConfigs[svc]
			alts := altPaths[svc]
			var foundAlt string
			for _, alt := range alts {
				if _, err := os.Stat(alt); err == nil {
					foundAlt = alt
					break
				}
			}
			fixHint := ""
			if foundAlt != "" {
				fixHint = fmt.Sprintf("Config found at alternative path: %s.", foundAlt)
			} else if svc == "seerr" {
				fixHint = fmt.Sprintf("Read from Docker: %s. Check container is running.", configPath)
			} else {
				fixHint = fmt.Sprintf("Expected config at: %s. Or get the API key from the %s web UI: Settings > General > API Key.", configPath, svc)
			}
			r.Issues = append(r.Issues, Issue{fmt.Sprintf("API KEY MISSING: %s API key not found. %s", svc, fixHint)})
			extra := ""
			if foundAlt != "" {
				extra = " " + ui.Dim("(found at "+foundAlt+")")
			}
			fmt.Printf("  %s %-15s %s%s\n", ui.Err("✗"), svc, ui.Err("not found"), extra)
		}
	}
}

func checkConfigFiles(r *Result) {
	fmt.Println(ui.Bold("\n  Config Files"))
	fmt.Println(ui.Separator())

	configs := map[string]string{
		"Sonarr":   "/mnt/c/ProgramData/Sonarr/config.xml",
		"Radarr":   "/mnt/c/ProgramData/Radarr/config.xml",
		"Prowlarr": "/mnt/c/ProgramData/Prowlarr/config.xml",
		"Plex":     "/mnt/c/Users/Max/AppData/Local/Plex/Plex Media Server/Preferences.xml",
		"Tautulli": "/mnt/c/ProgramData/Tautulli/config.ini",
	}
	configAlts := map[string][]string{
		"Tautulli": {
			"/mnt/c/Users/Max/AppData/Local/Tautulli/config.ini",
			"/mnt/c/Users/Max/AppData/Roaming/Tautulli/config.ini",
		},
		"Sonarr": {"/mnt/c/Users/Max/AppData/Roaming/Sonarr/config.xml"},
		"Radarr": {"/mnt/c/Users/Max/AppData/Roaming/Radarr/config.xml"},
	}

	for _, name := range []string{"Sonarr", "Radarr", "Prowlarr", "Plex", "Tautulli"} {
		path := configs[name]
		if _, err := os.Stat(path); err == nil {
			r.ChecksPassed++
			fmt.Printf("  %s %-15s %s\n", ui.Ok("✓"), name, ui.Dim(path))
		} else {
			alts := configAlts[name]
			var foundAlt string
			for _, alt := range alts {
				if _, err := os.Stat(alt); err == nil {
					foundAlt = alt
					break
				}
			}
			if foundAlt != "" {
				r.ChecksPassed++
				r.Issues = append(r.Issues, Issue{
					fmt.Sprintf("CONFIG ALTERNATE: %s config not at expected %s but found at %s.", name, path, foundAlt),
				})
				fmt.Printf("  %s %-15s %s %s\n", ui.Warn("!"), name, ui.Warn("alt path"), ui.Dim(foundAlt))
			} else {
				r.Issues = append(r.Issues, Issue{
					fmt.Sprintf("CONFIG MISSING: %s config not found at %s. Ensure %s is installed on Windows.", name, path, name),
				})
				fmt.Printf("  %s %-15s %s %s\n", ui.Err("✗"), name, ui.Err("missing"), ui.Dim(path))
			}
		}
	}
}

func checkDockerContainers(r *Result) {
	fmt.Println(ui.Bold("\n  Docker Containers"))
	fmt.Println(ui.Separator())

	out, err := exec.Command("docker", "ps", "-a", "--format", "{{.Names}}\t{{.Status}}\t{{.Image}}",
		"--filter", "name=seerr", "--filter", "name=bazarr",
		"--filter", "name=organizr", "--filter", "name=flaresolverr").Output()
	if err != nil {
		r.Issues = append(r.Issues, Issue{"DOCKER UNAVAILABLE: Docker CLI not found or not accessible."})
		fmt.Printf("  %s Docker not accessible\n", ui.Err("✗"))
		return
	}

	lines := strings.TrimSpace(string(out))
	if lines == "" {
		r.Issues = append(r.Issues, Issue{"NO CONTAINERS: No media stack Docker containers found."})
		fmt.Printf("  %s No media containers found\n", ui.Err("✗"))
		return
	}

	for _, line := range strings.Split(lines, "\n") {
		parts := strings.SplitN(line, "\t", 3)
		cname := "?"
		status := "?"
		image := "?"
		if len(parts) > 0 {
			cname = parts[0]
		}
		if len(parts) > 1 {
			status = parts[1]
		}
		if len(parts) > 2 {
			image = parts[2]
		}
		if strings.Contains(status, "Up") {
			r.ChecksPassed++
			fmt.Printf("  %s %-15s %s %s\n", ui.Ok("✓"), cname, ui.Ok(status), ui.Dim(image))
		} else {
			r.Issues = append(r.Issues, Issue{
				fmt.Sprintf("CONTAINER DOWN: '%s' (image: %s) status: %s. Fix: docker start %s", cname, image, status, cname),
			})
			fmt.Printf("  %s %-15s %s %s\n", ui.Err("✗"), cname, ui.Err(status), ui.Dim(image))
		}
	}
}

func checkDiskSpace(r *Result) {
	fmt.Println(ui.Bold("\n  Disk Space"))
	fmt.Println(ui.Separator())

	mediaWSL := config.MediaPathWSL()
	mediaWin := config.MediaPathWin()
	total, free, err := statfs(mediaWSL)
	if err != nil {
		r.Issues = append(r.Issues, Issue{
			fmt.Sprintf("DISK INACCESSIBLE: Cannot read %s. Check if drive is mounted.", mediaWSL),
		})
		fmt.Printf("  %s Cannot access %s\n", ui.Err("✗"), mediaWSL)
		return
	}

	pctUsed := float64(total-free) / float64(total) * 100

	if pctUsed > 95 {
		r.Issues = append(r.Issues, Issue{
			fmt.Sprintf("DISK CRITICAL: %s is %.0f%% full, only %s free of %s.", mediaWin, pctUsed, ui.FmtSize(free), ui.FmtSize(total)),
		})
		fmt.Printf("  %s %s  %s — %s free / %s\n", ui.Err("✗"), mediaWin, ui.Err(fmt.Sprintf("%.0f%% used", pctUsed)), ui.FmtSize(free), ui.FmtSize(total))
	} else if pctUsed > 85 {
		r.Issues = append(r.Issues, Issue{
			fmt.Sprintf("DISK LOW: %s is %.0f%% full, %s free of %s.", mediaWin, pctUsed, ui.FmtSize(free), ui.FmtSize(total)),
		})
		fmt.Printf("  %s %s  %s — %s free / %s\n", ui.Warn("!"), mediaWin, ui.Warn(fmt.Sprintf("%.0f%% used", pctUsed)), ui.FmtSize(free), ui.FmtSize(total))
	} else {
		r.ChecksPassed++
		fmt.Printf("  %s %s  %s — %s free / %s\n", ui.Ok("✓"), mediaWin, ui.Ok(fmt.Sprintf("%.0f%% used", pctUsed)), ui.FmtSize(free), ui.FmtSize(total))
	}
}

func checkMediaPaths(r *Result) {
	fmt.Println(ui.Bold("\n  Media Paths"))
	fmt.Println(ui.Separator())

	mediaWSL := config.MediaPathWSL()
	folders := []string{"Movies", "TV Shows", "Downloads", "Downloads/movies", "Downloads/tv"}

	for _, folder := range folders {
		path := filepath.Join(mediaWSL, folder)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			r.ChecksPassed++
			fmt.Printf("  %s %s\n", ui.Ok("✓"), path)
		} else {
			// Check for case-insensitive match
			parent := filepath.Dir(path)
			basename := filepath.Base(path)
			var caseMatch string
			if entries, err := os.ReadDir(parent); err == nil {
				for _, e := range entries {
					if strings.EqualFold(e.Name(), basename) && e.Name() != basename {
						caseMatch = filepath.Join(parent, e.Name())
						break
					}
				}
			}

			if caseMatch != "" {
				r.ChecksPassed++
				r.Issues = append(r.Issues, Issue{
					fmt.Sprintf("PATH CASE MISMATCH: Expected '%s' but found '%s' (different case).", path, caseMatch),
				})
				fmt.Printf("  %s %s %s %s\n", ui.Warn("!"), path, ui.Warn("→"), ui.Dim("found as: "+filepath.Base(caseMatch)))
			} else {
				r.Issues = append(r.Issues, Issue{
					fmt.Sprintf("PATH MISSING: Directory '%s' does not exist. Fix: mkdir -p \"%s\".", path, path),
				})
				fmt.Printf("  %s %s %s\n", ui.Err("✗"), path, ui.Err("missing"))
			}
		}
	}
}

func checkRootFolders(r *Result) {
	fmt.Println(ui.Bold("\n  Root Folders"))
	fmt.Println(ui.Separator())

	for _, item := range []struct {
		svc      string
		expected string
	}{
		{"radarr", "Movies"},
		{"sonarr", "TV Shows"},
	} {
		ver := config.ServiceAPIVer(item.svc)
		var roots []struct {
			Path       string `json:"path"`
			Accessible bool   `json:"accessible"`
			FreeSpace  int64  `json:"freeSpace"`
		}
		if err := api.GetJSON(item.svc, fmt.Sprintf("api/%s/rootfolder", ver), nil, &roots); err == nil {
			for _, root := range roots {
				if root.Accessible {
					r.ChecksPassed++
					fmt.Printf("  %s %s %s  %s\n", ui.Ok("✓"), ui.Dim("["+item.svc+"]"), root.Path, ui.Dim(ui.FmtSize(root.FreeSpace)+" free"))
				} else {
					wslEquiv := strings.ReplaceAll(root.Path, "\\", "/")
					if strings.HasPrefix(wslEquiv, "D:") {
						wslEquiv = "/mnt/d" + wslEquiv[2:]
					} else if strings.HasPrefix(wslEquiv, "C:") {
						wslEquiv = "/mnt/c" + wslEquiv[2:]
					}
					r.Issues = append(r.Issues, Issue{
						fmt.Sprintf("ROOT FOLDER INACCESSIBLE: %s root folder '%s' is not accessible. WSL equivalent: %s.", item.svc, root.Path, wslEquiv),
					})
					fmt.Printf("  %s %s %s %s %s\n", ui.Err("✗"), ui.Dim("["+item.svc+"]"), root.Path, ui.Err("inaccessible"), ui.Dim("→ "+wslEquiv))
				}
			}
		} else {
			fmt.Printf("  %s %s %s\n", ui.Dim("—"), ui.Dim("["+item.svc+"]"), ui.Dim("cannot check (service down)"))
		}
	}
}

func checkIndexers(r *Result) {
	fmt.Println(ui.Bold("\n  Indexers"))
	fmt.Println(ui.Separator())

	var indexers []struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Enable bool   `json:"enable"`
	}
	if err := api.GetJSON("prowlarr", "api/v1/indexer", nil, &indexers); err != nil {
		fmt.Printf("  %s %s\n", ui.Dim("—"), ui.Dim("Cannot check (Prowlarr down)"))
		return
	}

	var statuses []struct {
		IndexerID         int    `json:"indexerId"`
		MostRecentFailure string `json:"mostRecentFailure"`
		DisabledTill      string `json:"disabledTill"`
	}
	_ = api.GetJSON("prowlarr", "api/v1/indexerstatus", nil, &statuses)

	failedMap := make(map[int]string)
	for _, s := range statuses {
		if s.MostRecentFailure != "" {
			failedMap[s.IndexerID] = s.DisabledTill
		}
	}

	var healthy, failing, disabled []string
	for _, idx := range indexers {
		if !idx.Enable {
			disabled = append(disabled, idx.Name)
		} else if _, ok := failedMap[idx.ID]; ok {
			failing = append(failing, idx.Name)
		} else {
			healthy = append(healthy, idx.Name)
		}
	}

	if len(healthy) > 0 {
		r.ChecksPassed++
		fmt.Printf("  %s %d indexer(s) healthy: %s\n", ui.Ok("✓"), len(healthy), strings.Join(healthy, ", "))
	}
	if len(failing) > 0 {
		r.Issues = append(r.Issues, Issue{
			fmt.Sprintf("INDEXERS FAILING: %d indexer(s) failing: %s. Check Prowlarr.", len(failing), strings.Join(failing, ", ")),
		})
		fmt.Printf("  %s %d failing: %s\n", ui.Err("✗"), len(failing), strings.Join(failing, ", "))
	}
	if len(disabled) > 0 {
		fmt.Printf("  %s %d disabled: %s\n", ui.Dim("—"), len(disabled), strings.Join(disabled, ", "))
	}
}

func checkServiceWarnings(r *Result) {
	fmt.Println(ui.Bold("\n  Service Warnings"))
	fmt.Println(ui.Separator())

	found := false
	for _, svc := range []string{"radarr", "sonarr", "prowlarr"} {
		ver := config.ServiceAPIVer(svc)
		var data []struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			WikiURL string `json:"wikiUrl"`
			Source  string `json:"source"`
		}
		if err := api.GetJSON(svc, fmt.Sprintf("api/%s/health", ver), nil, &data); err == nil && len(data) > 0 {
			for _, item := range data {
				found = true
				level := ui.Warn("WARN")
				if item.Type == "error" {
					level = ui.Err("ERROR")
				}
				fmt.Printf("  %s %s %s\n", level, ui.Dim("["+svc+"]"), item.Message)
				if item.WikiURL != "" {
					fmt.Printf("         %s\n", ui.Dim(item.WikiURL))
				}
				r.Issues = append(r.Issues, Issue{
					fmt.Sprintf("HEALTH WARNING [%s] (%s): %s. Service: http://%s:%d/",
						svc, item.Type, item.Message, config.ServiceHost(svc), config.ServicePort(svc)),
				})
			}
		}
	}
	if !found {
		r.ChecksPassed++
		fmt.Printf("  %s No warnings from Radarr, Sonarr, or Prowlarr\n", ui.Ok("✓"))
	}
}
