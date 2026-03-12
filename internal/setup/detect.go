package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/ui"
)

// Detect runs Phase 1: environment detection.
func Detect(state *SetupState) StepResult {
	r := StepResult{Name: "Environment Detection"}

	fmt.Println(ui.Bold("\n  Phase 1 — Environment Detection"))
	fmt.Println(ui.Separator())

	// 1. Detect Windows host IP
	detectHost(state, &r)

	// 2. Detect media path
	detectMediaPath(state, &r)

	// 3. Detect installed services
	detectServices(state, &r)

	return r
}

func detectHost(state *SetupState, r *StepResult) {
	currentHost := config.Host()

	// Try /etc/resolv.conf nameserver (WSL2 pattern)
	detected := ""
	data, err := os.ReadFile("/etc/resolv.conf")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "nameserver") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					detected = parts[1]
					break
				}
			}
		}
	}

	// The resolv.conf IP is the WSL gateway, not necessarily the Windows host.
	// The current config default (192.168.50.42) is likely the actual Windows LAN IP.
	// Confirm with user.
	if detected != "" && detected != currentHost {
		fmt.Printf("  %s Detected gateway: %s (current config: %s)\n", ui.Dim("?"), detected, currentHost)
	}

	var host string
	var confirm bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Windows host IP is %s — correct?", currentHost)).
				Value(&confirm),
		),
	)
	if err := form.Run(); err != nil {
		state.Host = currentHost
		r.pass()
		return
	}

	if confirm {
		host = currentHost
	} else {
		form = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Enter Windows host IP").
					Placeholder("192.168.x.x").
					Value(&host),
			),
		)
		if err := form.Run(); err != nil || host == "" {
			host = currentHost
		}
	}

	state.Host = host
	fmt.Printf("  %s Host: %s\n", ui.Ok("✓"), state.Host)
	r.pass()
}

func detectMediaPath(state *SetupState, r *StepResult) {
	currentWSL := config.MediaPathWSL()
	currentWin := config.MediaPathWin()

	// Scan /mnt for media directories
	var candidates []string
	entries, err := os.ReadDir("/mnt")
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() || len(e.Name()) != 1 {
				continue // only single-letter mount points (c, d, e, ...)
			}
			mediaPath := filepath.Join("/mnt", e.Name(), "Media")
			if info, err := os.Stat(mediaPath); err == nil && info.IsDir() {
				candidates = append(candidates, mediaPath)
			}
		}
	}

	// Check if current config path is valid
	if _, err := os.Stat(currentWSL); err == nil {
		found := false
		for _, c := range candidates {
			if c == currentWSL {
				found = true
				break
			}
		}
		if !found {
			candidates = append([]string{currentWSL}, candidates...)
		}
	}

	if len(candidates) == 1 {
		state.MediaWSL = candidates[0]
	} else if len(candidates) > 1 {
		options := make([]huh.Option[string], len(candidates))
		for i, c := range candidates {
			options[i] = huh.NewOption(c, c)
		}
		var selected string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select media path").
					Options(options...).
					Value(&selected),
			),
		)
		if err := form.Run(); err != nil || selected == "" {
			state.MediaWSL = currentWSL
		} else {
			state.MediaWSL = selected
		}
	} else {
		// No candidates found
		var path string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Enter media path (WSL)").
					Placeholder("/mnt/d/Media").
					Value(&path),
			),
		)
		if err := form.Run(); err != nil || path == "" {
			state.MediaWSL = currentWSL
		} else {
			state.MediaWSL = path
		}
	}

	// Derive Windows path from WSL path
	// /mnt/d/Media -> D:\Media
	if strings.HasPrefix(state.MediaWSL, "/mnt/") && len(state.MediaWSL) > 6 {
		drive := strings.ToUpper(string(state.MediaWSL[5]))
		rest := strings.ReplaceAll(state.MediaWSL[6:], "/", `\`)
		state.MediaWin = drive + ":" + rest
	} else {
		state.MediaWin = currentWin
	}

	fmt.Printf("  %s Media: %s (%s)\n", ui.Ok("✓"), state.MediaWSL, state.MediaWin)
	r.pass()
}

func detectServices(state *SetupState, r *StepResult) {
	// Initialize from config defaults
	for _, name := range config.AllServiceNames() {
		svc := config.Get().Services[name]
		host := svc.Host
		if host == "" || host == config.Host() {
			host = state.Host
		}
		state.Services[name] = &ServiceState{
			Host: host,
			Port: svc.Port,
			Type: svc.Type,
		}
	}

	// Detect Windows processes
	out, err := exec.Command("/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe",
		"-Command", "Get-Process | Select-Object Name | Format-Table -HideTableHeaders").Output()
	if err == nil {
		procs := strings.ToLower(string(out))
		winProcs := map[string]string{
			"sonarr": "sonarr", "radarr": "radarr", "prowlarr": "prowlarr",
			"plex": "plex media server", "tautulli": "tautulli", "qbittorrent": "qbittorrent",
		}
		for svc, proc := range winProcs {
			if strings.Contains(procs, proc) {
				if s, ok := state.Services[svc]; ok {
					s.Detected = true
				}
			}
		}
	}

	// Detect Docker containers
	out, err = exec.Command("docker", "ps", "-a", "--format", "{{.Names}}").Output()
	if err == nil {
		containers := strings.ToLower(string(out))
		for _, name := range []string{"seerr", "bazarr", "organizr", "flaresolverr"} {
			if strings.Contains(containers, name) {
				if s, ok := state.Services[name]; ok {
					s.Detected = true
				}
			}
		}
	}

	detected := 0
	for _, s := range state.Services {
		if s.Detected {
			detected++
		}
	}

	fmt.Printf("  %s Detected %d/%d services\n", ui.Ok("✓"), detected, len(state.Services))

	for _, name := range config.AllServiceNames() {
		s := state.Services[name]
		status := ui.Ok("✓")
		label := "detected"
		if !s.Detected {
			status = ui.Dim("—")
			label = "not found"
		}
		fmt.Printf("    %s %-15s %s\n", status, name, ui.Dim(label))
	}
	r.pass()
}
