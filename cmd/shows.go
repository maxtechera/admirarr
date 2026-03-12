package cmd

import (
	"fmt"
	"sort"

	"github.com/maxtechera/admirarr/internal/api"
	"github.com/maxtechera/admirarr/internal/ui"
	"github.com/spf13/cobra"
)

var showsCmd = &cobra.Command{
	Use:   "shows",
	Short: "List all TV shows in Sonarr",
	Long: `List all TV shows in Sonarr with episode counts and size.

Fetches the full series list from Sonarr and displays each with:
  - Episode count: episodes on disk / total episodes
  - Color-coded: green (complete), yellow (partial), red (none)
  - Title, year, and size on disk

API endpoints used:
  Sonarr   GET /api/v3/series`,
	Example: "  admirarr shows",
	Run:     runShows,
}

func init() {
	rootCmd.AddCommand(showsCmd)
}

func runShows(cmd *cobra.Command, args []string) {
	ui.PrintBanner()
	fmt.Println(ui.Bold("\n  Sonarr — TV Shows\n"))

	var data []struct {
		Title string `json:"title"`
		Year  int    `json:"year"`
		Stats struct {
			EpisodeFileCount int   `json:"episodeFileCount"`
			EpisodeCount     int   `json:"episodeCount"`
			SizeOnDisk       int64 `json:"sizeOnDisk"`
		} `json:"statistics"`
	}
	if err := api.GetJSON("sonarr", "api/v3/series", nil, &data); err != nil {
		fmt.Printf("  %s\n", ui.Err("Cannot reach Sonarr"))
		return
	}
	if len(data) == 0 {
		fmt.Printf("  %s\n", ui.Dim("No shows added"))
		return
	}

	sort.Slice(data, func(i, j int) bool { return data[i].Title < data[j].Title })

	for _, s := range data {
		have := s.Stats.EpisodeFileCount
		total := s.Stats.EpisodeCount
		pct := fmt.Sprintf("%d/%d", have, total)
		colorFn := ui.Err
		if have == total && total > 0 {
			colorFn = ui.Ok
		} else if have > 0 {
			colorFn = ui.Warn
		}
		size := ui.FmtSize(s.Stats.SizeOnDisk)
		fmt.Printf("  %12s  %s (%d) %s\n", colorFn(pct), s.Title, s.Year, ui.Dim(size))
	}
	fmt.Printf("\n  %s\n\n", ui.Dim(fmt.Sprintf("%d shows total", len(data))))
}
