package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/ui"
)

// WriteConfig runs Phase 7: write validated config to disk.
func WriteConfig(state *SetupState) StepResult {
	r := StepResult{Name: "Write Config"}

	fmt.Println(ui.Bold("\n  Phase 7 — Write Config"))
	fmt.Println(ui.Separator())

	configDir := filepath.Join(os.Getenv("HOME"), ".config", "admirarr")
	configPath := filepath.Join(configDir, "config.yaml")

	// Build YAML content
	yaml := buildYAML(state)

	// Preview
	fmt.Printf("  %s %s\n", ui.Bold("Config preview:"), ui.Dim(configPath))
	fmt.Printf("  %s\n", ui.Dim("──────────────────────────────────────────────────"))
	for _, line := range strings.Split(yaml, "\n") {
		fmt.Printf("  %s %s\n", ui.Dim("│"), line)
	}
	fmt.Printf("  %s\n", ui.Dim("──────────────────────────────────────────────────"))

	// Check if file exists
	action := "write"
	if _, err := os.Stat(configPath); err == nil {
		var selected string
		form := huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title("Config file exists. What to do?").
				Options(
					huh.NewOption("Overwrite", "overwrite"),
					huh.NewOption("Skip", "skip"),
				).
				Value(&selected),
		))
		if err := form.Run(); err != nil || selected == "skip" {
			fmt.Printf("  %s Config write skipped\n", ui.Dim("—"))
			r.skip()
			return r
		}
		action = selected
	} else {
		var confirm bool
		form := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title("Write config?").
				Value(&confirm),
		))
		if err := form.Run(); err != nil || !confirm {
			r.skip()
			return r
		}
	}

	if action == "skip" {
		r.skip()
		return r
	}

	// Ensure directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		r.errf("cannot create config directory: %v", err)
		return r
	}

	if err := os.WriteFile(configPath, []byte(yaml), 0644); err != nil {
		r.errf("cannot write config: %v", err)
		return r
	}

	fmt.Printf("  %s Written to %s\n", ui.Ok("✓"), configPath)
	r.fix()
	return r
}

func buildYAML(state *SetupState) string {
	host := state.Host
	if host == "" {
		host = config.Host()
	}
	mediaWSL := state.MediaWSL
	if mediaWSL == "" {
		mediaWSL = config.MediaPathWSL()
	}
	mediaWin := state.MediaWin
	if mediaWin == "" {
		mediaWin = config.MediaPathWin()
	}

	var b strings.Builder

	b.WriteString("# ─── Admirarr Configuration ───────────────────────────────────────────\n")
	b.WriteString("# Run `admirarr setup` to converge your stack to this desired state.\n\n")

	b.WriteString(fmt.Sprintf("host: \"%s\"\n", host))
	b.WriteString("wsl_gateway: \"auto\"\n\n")

	b.WriteString("media:\n")
	b.WriteString(fmt.Sprintf("  wsl: \"%s\"\n", mediaWSL))
	b.WriteString(fmt.Sprintf("  win: \"%s\"\n\n", escapeBackslash(mediaWin)))

	// Services
	b.WriteString("# ─── Services ─────────────────────────────────────────────────────────\n")
	b.WriteString("services:\n")
	for _, name := range config.AllServiceNames() {
		svc := state.Services[name]
		if svc == nil {
			continue
		}
		b.WriteString(fmt.Sprintf("  %s:\n", name))
		b.WriteString(fmt.Sprintf("    port: %d\n", svc.Port))
		if svc.Type != "" {
			b.WriteString(fmt.Sprintf("    type: %s\n", svc.Type))
		}
		if svc.Type == "docker" {
			b.WriteString("    host: localhost\n")
		}
	}

	// Keys
	b.WriteString("\n# ─── API Keys ─────────────────────────────────────────────────────────\n")
	b.WriteString("keys:\n")
	for _, name := range []string{"sonarr", "radarr", "prowlarr", "plex", "tautulli", "seerr"} {
		key := ""
		if state.ManualKeys != nil {
			key = state.ManualKeys[name]
		}
		b.WriteString(fmt.Sprintf("  %s: \"%s\"\n", name, key))
	}

	// Quality profile
	qp := config.QualityProfile()
	if qp == "" {
		qp = "HD-1080p"
	}
	b.WriteString(fmt.Sprintf("\n# ─── Quality ──────────────────────────────────────────────────────────\n"))
	b.WriteString(fmt.Sprintf("quality_profile: \"%s\"\n", qp))

	// Indexers
	indexers := config.GetIndexers()
	if len(indexers) > 0 {
		b.WriteString("\n# ─── Indexers ─────────────────────────────────────────────────────────\n")
		b.WriteString("# Indexers NOT listed here will be removed during sync.\n")
		b.WriteString("indexers:\n")
		for name, ic := range indexers {
			if ic.BaseURL == "" && !ic.Flare && len(ic.ExtraFields) == 0 {
				b.WriteString(fmt.Sprintf("  %q: {}\n", name))
			} else {
				b.WriteString(fmt.Sprintf("  %q:\n", name))
				if ic.Flare {
					b.WriteString("    flare: true\n")
				}
				if ic.BaseURL != "" {
					b.WriteString(fmt.Sprintf("    base_url: \"%s\"\n", ic.BaseURL))
				}
				if len(ic.ExtraFields) > 0 {
					b.WriteString("    extra_fields:\n")
					for k, v := range ic.ExtraFields {
						b.WriteString(fmt.Sprintf("      %s: \"%v\"\n", k, v))
					}
				}
			}
		}
	}

	return b.String()
}

func escapeBackslash(s string) string {
	return strings.ReplaceAll(s, `\`, `\\`)
}
