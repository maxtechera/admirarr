package cmd

import (
	"fmt"
	"github.com/maxtechera/admirarr/internal/api"
	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/ui"
	"github.com/spf13/cobra"
)

var queueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Radarr + Sonarr import queues",
	Long: `Show Radarr and Sonarr import queues.

Displays items waiting to be imported with title, status, and warnings.

API endpoints used:
  Radarr   GET /api/v3/queue?pageSize=50
  Sonarr   GET /api/v3/queue?pageSize=50`,
	Example: "  admirarr queue",
	Run:     runQueue,
}

func init() {
	rootCmd.AddCommand(queueCmd)
}

func runQueue(cmd *cobra.Command, args []string) {
	ui.PrintBanner()
	fmt.Println(ui.Bold("\n  Download Queues\n"))

	for _, svc := range []string{"radarr", "sonarr"} {
		ver := config.ServiceAPIVer(svc)
		var data struct {
			TotalRecords int `json:"totalRecords"`
			Records      []struct {
				Title                string `json:"title"`
				TrackedDownloadState string `json:"trackedDownloadState"`
				StatusMessages       []struct {
					Messages []string `json:"messages"`
				} `json:"statusMessages"`
			} `json:"records"`
		}
		err := api.GetJSON(svc, fmt.Sprintf("api/%s/queue", ver), map[string]string{"pageSize": "50"}, &data)
		total := 0
		if err == nil {
			total = data.TotalRecords
		}
		svcTitle := svc
		if len(svc) > 0 {
			svcTitle = string(svc[0]-32) + svc[1:]
		}
		fmt.Println(ui.Bold(fmt.Sprintf("  %s (%d items)", svcTitle, total)))
		if total > 0 {
			for _, rec := range data.Records {
				state := rec.TrackedDownloadState
				colorFn := ui.Err
				if state == "downloading" {
					colorFn = ui.Ok
				} else if state == "importPending" {
					colorFn = ui.Warn
				}
				title := rec.Title
				if len(title) > 70 {
					title = title[:70]
				}
				fmt.Printf("    %s  %s\n", colorFn(state), title)
				for _, sm := range rec.StatusMessages {
					for i, m := range sm.Messages {
						if i >= 2 {
							break
						}
						msg := m
						if len(msg) > 80 {
							msg = msg[:80]
						}
						fmt.Printf("      %s\n", ui.Dim(msg))
					}
				}
			}
		} else {
			fmt.Printf("    %s\n", ui.Dim("Empty"))
		}
		fmt.Println()
	}
}
