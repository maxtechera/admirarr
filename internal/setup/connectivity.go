package setup

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/maxtechera/admirarr/internal/api"
	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/ui"
)

// CheckConnectivity runs Phase 2: service reachability and restart offers.
func CheckConnectivity(state *SetupState) StepResult {
	r := StepResult{Name: "Service Connectivity"}

	fmt.Println(ui.Bold("\n  Phase 2 — Service Connectivity"))
	fmt.Println(ui.Separator())

	for _, name := range config.AllServiceNames() {
		svc := state.Services[name]
		if !svc.Detected {
			fmt.Printf("  %s %-15s %s\n", ui.Dim("—"), name, ui.Dim("skipped (not detected)"))
			r.skip()
			continue
		}

		if api.CheckReachable(name) {
			svc.Reachable = true
			fmt.Printf("  %s %-15s %s\n", ui.Ok("✓"), name, ui.Ok("reachable"))
			r.pass()
			continue
		}

		fmt.Printf("  %s %-15s %s\n", ui.Err("✗"), name, ui.Err("unreachable"))

		// Offer to restart
		var confirm bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("%s is unreachable. Attempt to start/restart?", name)).
					Value(&confirm),
			),
		)
		if err := form.Run(); err != nil || !confirm {
			r.errf("%s unreachable (user skipped restart)", name)
			continue
		}

		if restartService(name, svc.Type) {
			// Wait and retry with backoff
			reachable := false
			for _, wait := range []time.Duration{1, 2, 4} {
				time.Sleep(wait * time.Second)
				if api.CheckReachable(name) {
					reachable = true
					break
				}
			}
			if reachable {
				svc.Reachable = true
				fmt.Printf("  %s %-15s %s\n", ui.Ok("✓"), name, ui.GoldText("started"))
				r.fix()
			} else {
				r.errf("%s still unreachable after restart", name)
			}
		} else {
			r.errf("%s restart failed", name)
		}
	}

	return r
}

var winServiceMap = map[string]string{
	"sonarr": "Sonarr", "radarr": "Radarr", "prowlarr": "Prowlarr",
	"plex": "Plex Media Server", "tautulli": "Tautulli",
}

func restartService(name, svcType string) bool {
	if svcType == "docker" {
		out, err := exec.Command("docker", "start", name).CombinedOutput()
		if err != nil {
			fmt.Printf("    %s %s\n", ui.Err("Failed:"), strings.TrimSpace(string(out)))
			return false
		}
		fmt.Printf("    %s docker start %s\n", ui.Dim("→"), name)
		return true
	}

	// Windows service or desktop app
	if winSvc, ok := winServiceMap[name]; ok {
		psCmd := fmt.Sprintf(`Start-Service '%s'`, winSvc)
		cmd := exec.Command("/mnt/c/Windows/System32/cmd.exe", "/c",
			fmt.Sprintf(`powershell -Command "%s"`, psCmd))
		_ = cmd.Run()
		fmt.Printf("    %s powershell Start-Service '%s'\n", ui.Dim("→"), winSvc)
		return true
	}

	// Desktop apps (qbittorrent, tautulli) - can't easily start from WSL
	fmt.Printf("    %s %s is a desktop app — please start it from Windows\n", ui.Warn("!"), name)
	return false
}
