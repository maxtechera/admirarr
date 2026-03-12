package cmd

import (
	"fmt"

	"github.com/maxtechera/admirarr/internal/api"
	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/ui"
	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Health warnings from Radarr, Sonarr, and Prowlarr",
	Long: `Show health warnings from Radarr, Sonarr, and Prowlarr.

Queries the health endpoint of each *Arr service and displays warnings.
Shows ERROR or WARN level messages with the originating service.

API endpoints used:
  Radarr     GET /api/v3/health
  Sonarr     GET /api/v3/health
  Prowlarr   GET /api/v1/health`,
	Example: "  admirarr health",
	Run:     runHealth,
}

func init() {
	rootCmd.AddCommand(healthCmd)
}

func runHealth(cmd *cobra.Command, args []string) {
	ui.PrintBanner()
	fmt.Println(ui.Bold("\n  Health Check\n"))

	for _, svc := range []string{"radarr", "sonarr", "prowlarr"} {
		ver := config.ServiceAPIVer(svc)
		var data []struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		}
		err := api.GetJSON(svc, fmt.Sprintf("api/%s/health", ver), nil, &data)
		if err != nil || len(data) == 0 {
			fmt.Printf("  %s %s Healthy\n", ui.Ok("●"), ui.Dim("["+svc+"]"))
			continue
		}
		for _, item := range data {
			level := ui.Warn("WARN")
			if item.Type == "error" {
				level = ui.Err("ERROR")
			}
			fmt.Printf("  %s %s %s\n", level, ui.Dim("["+svc+"]"), item.Message)
		}
	}
	fmt.Println()
}
