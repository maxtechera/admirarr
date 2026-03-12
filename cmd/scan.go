package cmd

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/keys"
	"github.com/maxtechera/admirarr/internal/ui"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Trigger Plex library scan",
	Long: `Trigger a Plex library scan across all sections.

Fetches all Plex library sections, then triggers a refresh on each one.

API endpoints used:
  Plex   GET  /library/sections
  Plex   POST /library/sections/<id>/refresh`,
	Example: "  admirarr scan",
	Run:     runScan,
}

func init() {
	rootCmd.AddCommand(scanCmd)
}

func runScan(cmd *cobra.Command, args []string) {
	ui.PrintBanner()
	fmt.Println(ui.Bold("\n  Triggering Plex Library Scan...\n"))

	key := keys.Get("plex")
	if key == "" {
		fmt.Printf("  %s\n", ui.Err("No Plex token found"))
		return
	}

	sectionsURL := fmt.Sprintf("%s/library/sections?X-Plex-Token=%s", config.ServiceURL("plex"), key)
	c := &http.Client{Timeout: 5 * time.Second}
	resp, err := c.Get(sectionsURL)
	if err != nil {
		fmt.Printf("  %s\n", ui.Err(fmt.Sprintf("Failed: %v", err)))
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var container struct {
		Directories []struct {
			Key   string `xml:"key,attr"`
			Title string `xml:"title,attr"`
		} `xml:"Directory"`
	}
	if err := xml.Unmarshal(body, &container); err != nil {
		fmt.Printf("  %s\n", ui.Err(fmt.Sprintf("Failed to parse: %v", err)))
		return
	}

	for _, d := range container.Directories {
		scanURL := fmt.Sprintf("%s/library/sections/%s/refresh?X-Plex-Token=%s",
			config.ServiceURL("plex"), d.Key, key)
		req, _ := http.NewRequest("POST", scanURL, strings.NewReader(""))
		_, err := c.Do(req)
		if err != nil {
			fmt.Printf("  %s Failed: %s — %v\n", ui.Err("●"), d.Title, err)
		} else {
			fmt.Printf("  %s Scanning: %s\n", ui.Ok("●"), d.Title)
		}
	}
	fmt.Println()
}
