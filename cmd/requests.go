package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/maxtechera/admirarr/internal/api"
	"github.com/maxtechera/admirarr/internal/ui"
	"github.com/spf13/cobra"
)

var requestsCmd = &cobra.Command{
	Use:   "requests",
	Short: "Seerr media requests",
	Long: `Show pending and recent media requests from Seerr.

Fetches all requests and resolves titles via the Seerr media API.

Status: PENDING (awaiting approval), APPROVED (searching/downloading),
AVAILABLE (on disk), DECLINED, PROCESSING.

API endpoints used:
  Seerr   GET /api/v1/request?take=20
  Seerr   GET /api/v1/movie/<tmdbId>  or  /api/v1/tv/<tmdbId>`,
	Example: "  admirarr requests",
	Run:     runRequests,
}

func init() {
	rootCmd.AddCommand(requestsCmd)
}

var statusNames = map[int]string{
	1: "PENDING",
	2: "APPROVED",
	3: "DECLINED",
	4: "AVAILABLE",
	5: "PROCESSING",
}

func runRequests(cmd *cobra.Command, args []string) {
	ui.PrintBanner()

	var data struct {
		PageInfo struct {
			Results int `json:"results"`
		} `json:"pageInfo"`
		Results []struct {
			ID     int  `json:"id"`
			Status int  `json:"status"`
			Type   string `json:"type"`
			Is4K   bool `json:"is4k"`
			Media  struct {
				MediaType string `json:"mediaType"`
				TmdbID    int    `json:"tmdbId"`
			} `json:"media"`
			CreatedAt   string `json:"createdAt"`
			RequestedBy struct {
				DisplayName string `json:"displayName"`
			} `json:"requestedBy"`
		} `json:"results"`
	}

	if err := api.GetJSON("seerr", "api/v1/request", map[string]string{"take": "20"}, &data); err != nil {
		fmt.Printf("\n  %s\n\n", ui.Err("Cannot reach Seerr"))
		return
	}

	fmt.Printf("%s\n", ui.Bold(fmt.Sprintf("\n  Seerr — Requests (%d total)\n", data.PageInfo.Results)))

	if len(data.Results) == 0 {
		fmt.Printf("  %s\n\n", ui.Dim("No requests"))
		return
	}

	for _, r := range data.Results {
		status := statusNames[r.Status]
		if status == "" {
			status = fmt.Sprintf("?(%d)", r.Status)
		}

		// Resolve title from Seerr media endpoint
		title, year := resolveTitle(r.Media.MediaType, r.Media.TmdbID)

		icon := "○"
		colorFn := ui.Dim
		switch r.Status {
		case 4: // AVAILABLE
			icon = "●"
			colorFn = ui.Ok
		case 2: // APPROVED
			icon = "◐"
			colorFn = ui.Warn
		case 1: // PENDING
			icon = "○"
			colorFn = ui.GoldText
		case 3: // DECLINED
			icon = "✗"
			colorFn = ui.Err
		}

		suffix := ""
		if r.Is4K {
			suffix = " [4K]"
		}
		user := r.RequestedBy.DisplayName
		date := r.CreatedAt
		if len(date) > 10 {
			date = date[:10]
		}

		fmt.Printf("  %s %-12s %s (%s)%s  — %s, %s\n",
			colorFn(icon), colorFn(status), title, year, suffix, ui.Dim(user), ui.Dim(date))
	}
	fmt.Println()
}

func resolveTitle(mediaType string, tmdbID int) (string, string) {
	endpoint := fmt.Sprintf("api/v1/%s/%d", mediaType, tmdbID)
	var info map[string]interface{}
	if err := api.GetJSON("seerr", endpoint, nil, &info); err != nil {
		return fmt.Sprintf("TMDB:%d", tmdbID), "?"
	}

	title := "?"
	if t, ok := info["title"].(string); ok && t != "" {
		title = t
	} else if t, ok := info["name"].(string); ok && t != "" {
		title = t
	}

	year := "?"
	if d, ok := info["releaseDate"].(string); ok && len(d) >= 4 {
		year = d[:4]
	} else if d, ok := info["firstAirDate"].(string); ok && len(d) >= 4 {
		year = d[:4]
	}

	return title, year
}

// Ensure json is used (for the inline struct unmarshaling)
var _ = json.Unmarshal
