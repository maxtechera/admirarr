package cmd

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/keys"
	"github.com/maxtechera/admirarr/internal/ui"
	"github.com/spf13/cobra"
)

var recentCmd = &cobra.Command{
	Use:   "recent",
	Short: "Recently added to Plex",
	Long: `Show recently added content in Plex.

Queries Plex for the 15 most recently added items across all libraries.

API endpoints used:
  Plex   GET /library/recentlyAdded?X-Plex-Container-Size=15`,
	Example: "  admirarr recent",
	Run:     runRecent,
}

func init() {
	rootCmd.AddCommand(recentCmd)
}

func runRecent(cmd *cobra.Command, args []string) {
	ui.PrintBanner()
	fmt.Println(ui.Bold("\n  Plex — Recently Added\n"))

	key := keys.Get("plex")
	if key == "" {
		fmt.Printf("  %s\n", ui.Err("No Plex token found"))
		return
	}

	url := fmt.Sprintf("%s/library/recentlyAdded?X-Plex-Token=%s&X-Plex-Container-Start=0&X-Plex-Container-Size=15",
		config.ServiceURL("plex"), key)

	c := &http.Client{Timeout: 5 * time.Second}
	resp, err := c.Get(url)
	if err != nil {
		fmt.Printf("  %s\n", ui.Err(fmt.Sprintf("Failed: %v", err)))
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var container struct {
		Videos []struct {
			Title string `xml:"title,attr"`
			Year  string `xml:"year,attr"`
			Type  string `xml:"type,attr"`
		} `xml:"Video"`
		Directories []struct {
			Title string `xml:"title,attr"`
			Year  string `xml:"year,attr"`
			Type  string `xml:"type,attr"`
		} `xml:"Directory"`
	}
	if err := xml.Unmarshal(body, &container); err != nil {
		fmt.Printf("  %s\n", ui.Err(fmt.Sprintf("Failed to parse: %v", err)))
		return
	}

	if len(container.Videos) == 0 && len(container.Directories) == 0 {
		fmt.Printf("  %s\n", ui.Dim("Nothing recently added"))
		return
	}

	for _, v := range container.Videos {
		fmt.Printf("  %s %s (%s) — %s\n", ui.Ok("+"), v.Title, v.Year, v.Type)
	}
	for _, d := range container.Directories {
		fmt.Printf("  %s %s (%s) — %s\n", ui.Ok("+"), d.Title, d.Year, d.Type)
	}
	fmt.Println()
}
