package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/maxtechera/admirarr/internal/api"
	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/ui"
)

// ValidateRootFolders runs Phase 5: root folder and media path validation.
func ValidateRootFolders(state *SetupState) StepResult {
	r := StepResult{Name: "Root Folders & Media Paths"}

	fmt.Println(ui.Bold("\n  Phase 5 — Root Folders & Media Paths"))
	fmt.Println(ui.Separator())

	mediaWSL := state.MediaWSL
	if mediaWSL == "" {
		mediaWSL = config.MediaPathWSL()
	}
	mediaWin := state.MediaWin
	if mediaWin == "" {
		mediaWin = config.MediaPathWin()
	}

	// 5a. Create missing media directories
	fmt.Println(ui.Bold("  Media Directories"))

	requiredDirs := []string{"Movies", "TV Shows", "Downloads", "Downloads/movies", "Downloads/tv"}
	for _, dir := range requiredDirs {
		path := filepath.Join(mediaWSL, dir)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			fmt.Printf("  %s %s\n", ui.Ok("✓"), path)
			r.pass()
			continue
		}

		// Check case-insensitive match
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
			fmt.Printf("  %s %s → found as %s\n", ui.Warn("!"), path, ui.Dim(filepath.Base(caseMatch)))
			r.pass()
			continue
		}

		fmt.Printf("  %s %s %s\n", ui.Err("✗"), path, ui.Err("missing"))

		var confirm bool
		form := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Create %s?", path)).
				Value(&confirm),
		))
		if err := form.Run(); err != nil || !confirm {
			r.errf("directory %s does not exist", path)
			continue
		}

		if err := os.MkdirAll(path, 0755); err != nil {
			// Try PowerShell for Windows filesystem
			winPath := wslToWin(path)
			if _, err2 := execPowershell(fmt.Sprintf(`New-Item -Path '%s' -ItemType Directory -Force`, winPath)); err2 != nil {
				r.errf("cannot create %s: %v", path, err)
				continue
			}
		}
		fmt.Printf("  %s Created %s\n", ui.Ok("✓"), path)
		r.fix()
	}

	// 5b. Validate root folders in Radarr/Sonarr
	fmt.Printf("\n%s\n", ui.Bold("  Root Folders"))

	arrConfigs := []struct {
		service  string
		subdir   string
	}{
		{"radarr", "Movies"},
		{"sonarr", "TV Shows"},
	}

	for _, ac := range arrConfigs {
		svc := state.Services[ac.service]
		if svc == nil || !svc.Reachable {
			fmt.Printf("  %s [%s] %s\n", ui.Dim("—"), ac.service, ui.Dim("skipped (not reachable)"))
			r.skip()
			continue
		}

		ver := config.ServiceAPIVer(ac.service)
		expectedWin := mediaWin + `\` + ac.subdir

		var roots []struct {
			ID         int    `json:"id"`
			Path       string `json:"path"`
			Accessible bool   `json:"accessible"`
			FreeSpace  int64  `json:"freeSpace"`
		}
		if err := api.GetJSON(ac.service, fmt.Sprintf("api/%s/rootfolder", ver), nil, &roots); err != nil {
			r.errf("[%s] cannot query root folders: %v", ac.service, err)
			continue
		}

		// Check if expected root folder exists
		found := false
		for _, root := range roots {
			normalized := strings.ReplaceAll(root.Path, "/", `\`)
			if strings.EqualFold(normalized, expectedWin) {
				found = true
				if root.Accessible {
					fmt.Printf("  %s [%s] %s  %s\n", ui.Ok("✓"), ac.service, root.Path,
						ui.Dim(ui.FmtSize(root.FreeSpace)+" free"))
					r.pass()
				} else {
					fmt.Printf("  %s [%s] %s %s\n", ui.Err("✗"), ac.service, root.Path,
						ui.Err("inaccessible"))
					r.errf("[%s] root folder %s exists but is not accessible — check if the directory exists on disk", ac.service, root.Path)
				}
				break
			}
		}

		if !found {
			fmt.Printf("  %s [%s] Root folder %s not configured\n", ui.Err("✗"), ac.service, expectedWin)

			var confirm bool
			form := huh.NewForm(huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Add root folder %s to %s?", expectedWin, ac.service)).
					Value(&confirm),
			))
			if err := form.Run(); err != nil || !confirm {
				r.errf("[%s] root folder %s not configured", ac.service, expectedWin)
				continue
			}

			payload := map[string]string{"path": expectedWin}
			if _, err := api.Post(ac.service, fmt.Sprintf("api/%s/rootfolder", ver), payload, nil); err != nil {
				r.errf("[%s] failed to create root folder: %v", ac.service, err)
				continue
			}

			fmt.Printf("  %s [%s] Root folder %s added\n", ui.Ok("✓"), ac.service, expectedWin)
			r.fix()
		}
	}

	return r
}

func execPowershell(cmd string) (string, error) {
	out, err := exec.Command("/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe",
		"-Command", cmd).CombinedOutput()
	return string(out), err
}
