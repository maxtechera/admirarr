package cmd

import (
	"fmt"

	"github.com/maxtechera/admirarr/internal/setup"
	"github.com/maxtechera/admirarr/internal/ui"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup wizard — validate and configure your entire stack",
	Long: `Interactive setup wizard that validates and configures your entire
Plex + *Arr media server stack in one pass.

Runs 7 phases:
  1. Environment Detection  — host IP, media path, installed services
  2. Service Connectivity   — reachability checks, restart offers
  3. API Key Validation     — auto-discovery + validation
  4. Download Client Config — qBittorrent categories, save paths, *Arr clients
  5. Root Folders           — media directories + *Arr root folder config
  6. Indexer Verification   — Prowlarr indexers + sync targets
  7. Write Config           — save validated config to disk`,
	Example: "  admirarr setup",
	Run:     runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) {
	ui.PrintBanner()
	fmt.Println()
	fmt.Printf("  %s %s\n", ui.GoldText("⚓"), ui.Bold("Setup Wizard — configure your fleet"))
	fmt.Println(ui.Separator())

	setup.Run()
}
