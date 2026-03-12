package cmd

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/maxtechera/admirarr/internal/api"
	"github.com/maxtechera/admirarr/internal/ui"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search Prowlarr indexers for torrents",
	Long: `Search Prowlarr indexers for torrents matching a query.

Sends a search query to Prowlarr which fans out to all enabled indexers.
Results sorted by seeders, showing indexer, title, size, and seeders.

API endpoints used:
  Prowlarr   GET /api/v1/search?query=<query>&type=search`,
	Example: "  admirarr search interstellar\n  admirarr search \"the bear s03\"",
	Args:    cobra.MinimumNArgs(1),
	Run:     runSearch,
}

func init() {
	rootCmd.AddCommand(searchCmd)
}

func runSearch(cmd *cobra.Command, args []string) {
	query := strings.Join(args, " ")
	ui.PrintBanner()
	fmt.Printf("%s\n", ui.Bold(fmt.Sprintf("\n  Prowlarr Search: %s\n", query)))

	var data []struct {
		Title   string `json:"title"`
		Size    int64  `json:"size"`
		Seeders int    `json:"seeders"`
		Indexer string `json:"indexer"`
	}
	params := map[string]string{"query": query, "type": "search"}
	if err := api.GetJSON("prowlarr", "api/v1/search", params, &data, 30*time.Second); err != nil {
		fmt.Printf("  %s\n", ui.Err("No results or cannot reach Prowlarr"))
		return
	}
	if len(data) == 0 {
		fmt.Printf("  %s\n", ui.Dim("No results"))
		return
	}

	sort.Slice(data, func(i, j int) bool { return data[i].Seeders > data[j].Seeders })

	limit := 15
	if len(data) < limit {
		limit = len(data)
	}
	for i, r := range data[:limit] {
		title := r.Title
		if len(title) > 70 {
			title = title[:70]
		}
		size := ui.FmtSize(r.Size)
		seedColor := ui.Err
		if r.Seeders > 10 {
			seedColor = ui.Ok
		} else if r.Seeders > 0 {
			seedColor = ui.Warn
		}
		fmt.Printf("  %s %s\n", ui.Dim(fmt.Sprintf("%2d.", i+1)), title)
		fmt.Printf("      %s | %s | Seeds: %s\n", ui.Dim(r.Indexer), size, seedColor(fmt.Sprintf("%d", r.Seeders)))
	}
	fmt.Println()
}
