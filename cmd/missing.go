package cmd

import (
	"fmt"

	"github.com/maxtechera/admirarr/internal/api"
	"github.com/maxtechera/admirarr/internal/ui"
	"github.com/spf13/cobra"
)

var missingCmd = &cobra.Command{
	Use:   "missing",
	Short: "Monitored content without files",
	Long: `Show monitored content that is missing files.

Movies: from Radarr, filtered by monitored=true and hasFile=false.
Episodes: from Sonarr wanted/missing endpoint, sorted by air date.

API endpoints used:
  Radarr   GET /api/v3/movie
  Sonarr   GET /api/v3/wanted/missing?pageSize=20`,
	Example: "  admirarr missing",
	Run:     runMissing,
}

func init() {
	rootCmd.AddCommand(missingCmd)
}

func runMissing(cmd *cobra.Command, args []string) {
	ui.PrintBanner()
	fmt.Println(ui.Bold("\n  Missing Content\n"))

	// Movies
	var movies []struct {
		Title     string `json:"title"`
		Year      int    `json:"year"`
		HasFile   bool   `json:"hasFile"`
		Monitored bool   `json:"monitored"`
		Status    string `json:"status"`
	}
	if err := api.GetJSON("radarr", "api/v3/movie", nil, &movies); err == nil {
		var missing []int
		for i, m := range movies {
			if m.Monitored && !m.HasFile {
				missing = append(missing, i)
			}
		}
		fmt.Println(ui.Bold(fmt.Sprintf("  Movies (%d missing)", len(missing))))
		for _, i := range missing {
			m := movies[i]
			fmt.Printf("  %s %s (%d) — %s\n", ui.Err("○"), m.Title, m.Year, m.Status)
		}
	} else {
		fmt.Printf("  %s\n", ui.Err("Cannot reach Radarr"))
	}

	// Episodes
	fmt.Println()
	var eps struct {
		TotalRecords int `json:"totalRecords"`
		Records      []struct {
			Title         string `json:"title"`
			SeasonNumber  int    `json:"seasonNumber"`
			EpisodeNumber int    `json:"episodeNumber"`
			Series        struct {
				Title string `json:"title"`
			} `json:"series"`
		} `json:"records"`
	}
	params := map[string]string{
		"pageSize":      "20",
		"sortKey":       "airDateUtc",
		"sortDirection": "descending",
	}
	if err := api.GetJSON("sonarr", "api/v3/wanted/missing", params, &eps); err == nil {
		fmt.Println(ui.Bold(fmt.Sprintf("  Episodes (%d missing)", eps.TotalRecords)))
		limit := 15
		if len(eps.Records) < limit {
			limit = len(eps.Records)
		}
		for _, e := range eps.Records[:limit] {
			fmt.Printf("  %s %s S%02dE%02d — %s\n", ui.Err("○"), e.Series.Title, e.SeasonNumber, e.EpisodeNumber, e.Title)
		}
	} else {
		fmt.Printf("  %s\n", ui.Err("Cannot reach Sonarr"))
	}
	fmt.Println()
}
