package cmd

import (
	"fmt"

	"github.com/maxtechera/admirarr/internal/api"
	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/ui"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs <sonarr|radarr>",
	Short: "Recent logs from Sonarr or Radarr",
	Long: `View recent logs from Sonarr or Radarr.

Fetches the 20 most recent log entries from the specified service.

API endpoints used:
  Sonarr   GET /api/v3/log?pageSize=20&sortDirection=descending&sortKey=time
  Radarr   GET /api/v3/log?pageSize=20&sortDirection=descending&sortKey=time`,
	Example: "  admirarr logs sonarr\n  admirarr logs radarr",
	Args:    cobra.ExactArgs(1),
	Run:     runLogs,
}

func init() {
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) {
	service := args[0]
	if service != "sonarr" && service != "radarr" {
		fmt.Printf("  %s\n", ui.Err("Logs only available for sonarr or radarr"))
		return
	}

	ui.PrintBanner()
	fmt.Printf("%s\n", ui.Bold(fmt.Sprintf("\n  %s — Recent Logs\n", capitalize(service))))

	ver := config.ServiceAPIVer(service)
	var data struct {
		Records []struct {
			Level   string `json:"level"`
			Message string `json:"message"`
			Time    string `json:"time"`
		} `json:"records"`
	}
	params := map[string]string{
		"pageSize":      "20",
		"sortDirection": "descending",
		"sortKey":       "time",
	}
	if err := api.GetJSON(service, fmt.Sprintf("api/%s/log", ver), params, &data); err != nil {
		fmt.Printf("  %s\n", ui.Err(fmt.Sprintf("Cannot get %s logs", service)))
		return
	}

	for _, rec := range data.Records {
		msg := rec.Message
		if len(msg) > 100 {
			msg = msg[:100]
		}
		t := rec.Time
		if len(t) > 19 {
			t = t[:19]
		}
		colorFn := ui.Dim
		if rec.Level == "error" {
			colorFn = ui.Err
		} else if rec.Level == "warn" {
			colorFn = ui.Warn
		}
		fmt.Printf("  %s %8s  %s\n", ui.Dim(t), colorFn(rec.Level), msg)
	}
	fmt.Println()
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]-32) + s[1:]
}
