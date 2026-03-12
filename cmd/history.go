package cmd

import (
	"fmt"

	"github.com/maxtechera/admirarr/internal/api"
	"github.com/maxtechera/admirarr/internal/keys"
	"github.com/maxtechera/admirarr/internal/ui"
	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Tautulli watch history",
	Long: `Show Tautulli watch history.

Fetches recent watch history from Tautulli with user, title, and duration.

API endpoints used:
  Tautulli   GET /api/v2?cmd=get_history&length=10`,
	Example: "  admirarr history",
	Run:     runHistory,
}

func init() {
	rootCmd.AddCommand(historyCmd)
}

func runHistory(cmd *cobra.Command, args []string) {
	ui.PrintBanner()
	fmt.Println(ui.Bold("\n  Tautulli — Watch History\n"))

	key := keys.Get("tautulli")
	if key == "" {
		fmt.Printf("  %s\n", ui.Err("No Tautulli API key found"))
		return
	}

	var data struct {
		Response struct {
			Data struct {
				Data []struct {
					FullTitle string `json:"full_title"`
					User      string `json:"user"`
					Duration  int    `json:"duration"`
				} `json:"data"`
			} `json:"data"`
		} `json:"response"`
	}
	params := map[string]string{
		"apikey": key,
		"cmd":    "get_history",
		"length": "10",
	}
	if err := api.GetJSON("tautulli", "api/v2", params, &data); err != nil {
		fmt.Printf("  %s\n", ui.Err("Cannot reach Tautulli"))
		return
	}

	history := data.Response.Data.Data
	if len(history) == 0 {
		fmt.Printf("  %s\n", ui.Dim("No watch history yet"))
		return
	}

	for _, h := range history {
		dur := h.Duration / 60
		fmt.Printf("  %15s  %s %s\n", ui.GoldText(h.User), h.FullTitle, ui.Dim(fmt.Sprintf("(%d min)", dur)))
	}
	fmt.Println()
}
