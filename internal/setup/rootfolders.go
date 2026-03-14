package setup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/maxtechera/admirarr/internal/arr"
	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/ui"
)

// ValidateRootFolders runs Phase 6: root folder and media path validation.
func ValidateRootFolders(state *SetupState) StepResult {
	r := StepResult{Name: "Root Folders"}

	dataPath := state.DataPath
	if dataPath == "" {
		dataPath = config.DataPath()
	}

	// 6a. Create missing media directories (TRaSH Guides structure)
	fmt.Println(ui.Bold("  Media Directories"))

	// Build directory list based on selected services
	requiredDirs := []string{
		"media", "torrents",
	}
	for _, dc := range config.DefaultDownloadClients {
		if state.Services[dc.Service] != nil {
			requiredDirs = append(requiredDirs, dc.TorrentDir)
		}
	}
	for _, rf := range config.DefaultRootFolders {
		if state.Services[rf.Service] != nil {
			requiredDirs = append(requiredDirs, rf.Subdir)
		}
	}

	for _, dir := range requiredDirs {
		path := filepath.Join(dataPath, dir)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			fmt.Printf("  %s %s\n", ui.Ok("✓"), path)
			r.pass()
			continue
		}

		if state.wouldFix(&r, "Create directory %s", path) {
			continue
		}

		if err := os.MkdirAll(path, 0755); err != nil {
			r.errf("cannot create %s: %v", path, err)
			continue
		}
		fmt.Printf("  %s Created %s\n", ui.Ok("✓"), path)
		r.fix()
	}

	// 6b. Validate root folders in all *Arr services
	fmt.Printf("\n%s\n", ui.Bold("  Root Folders"))

	for _, rf := range config.DefaultRootFolders {
		svc := state.Services[rf.Service]
		if svc == nil || !svc.Reachable {
			fmt.Printf("  %s %s → %s\n", ui.Dim("—"), titleCase(rf.Service), ui.Dim("skipped (not reachable)"))
			r.skip()
			continue
		}

		client := arr.New(rf.Service)
		expectedPath := filepath.Join(dataPath, rf.Subdir)

		roots, err := client.RootFolders()
		if err != nil {
			r.errf("%s → cannot query root folders: %v", titleCase(rf.Service), err)
			continue
		}

		// Check if expected root folder exists
		found := false
		for _, root := range roots {
			if root.Path == expectedPath {
				found = true
				if root.Accessible {
					fmt.Printf("  %s %s → %s  %s\n", ui.Ok("✓"), titleCase(rf.Service), root.Path,
						ui.Dim(ui.FmtSize(root.FreeSpace)+" free"))
					r.pass()
				} else {
					fmt.Printf("  %s %s → %s %s\n", ui.Err("✗"), titleCase(rf.Service), root.Path,
						ui.Err("inaccessible"))
					r.errf("%s → root folder %s exists but is not accessible", titleCase(rf.Service), root.Path)
				}
				break
			}
		}

		if !found {
			// Auto-create root folder
			fmt.Printf("  %s %s → Adding root folder %s\n", ui.GoldText("↻"), titleCase(rf.Service), expectedPath)

			if state.wouldFix(&r, "%s → Add root folder %s", titleCase(rf.Service), expectedPath) {
				continue
			}

			if err := client.AddRootFolder(expectedPath); err != nil {
				r.errf("%s → failed to create root folder: %v", titleCase(rf.Service), err)
				continue
			}

			fmt.Printf("  %s %s → Root folder %s added\n", ui.Ok("✓"), titleCase(rf.Service), expectedPath)
			r.fix()
		}
	}

	return r
}
