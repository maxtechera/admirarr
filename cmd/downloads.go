package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/ui"
	"github.com/spf13/cobra"
)

var downloadsCmd = &cobra.Command{
	Use:   "downloads",
	Short: "Active qBittorrent torrents",
	Long: `Show active qBittorrent torrents with progress, speed, and ETA.

Queries qBittorrent Web API for all torrents and displays:
  - Progress percentage with visual bar
  - Download speed in MB/s
  - Torrent name and size

API endpoints used:
  qBit   GET /api/v2/torrents/info`,
	Example: "  admirarr downloads",
	Run:     runDownloads,
}

func init() {
	rootCmd.AddCommand(downloadsCmd)
}

func runDownloads(cmd *cobra.Command, args []string) {
	ui.PrintBanner()
	fmt.Println(ui.Bold("\n  qBittorrent — Downloads\n"))

	url := fmt.Sprintf("%s/api/v2/torrents/info", config.ServiceURL("qbittorrent"))
	c := &http.Client{Timeout: 5 * time.Second}
	resp, err := c.Get(url)
	if err != nil {
		fmt.Printf("  %s\n", ui.Err("Cannot reach qBittorrent"))
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var data []struct {
		Name     string  `json:"name"`
		Size     int64   `json:"size"`
		Progress float64 `json:"progress"`
		DLSpeed  int64   `json:"dlspeed"`
		State    string  `json:"state"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		fmt.Printf("  %s\n", ui.Err("Cannot parse qBittorrent response"))
		return
	}

	dlStates := map[string]bool{"downloading": true, "stalledDL": true, "forcedDL": true, "metaDL": true}
	seedStates := map[string]bool{"uploading": true, "stalledUP": true, "forcedUP": true}

	var downloading, seeding, paused []int
	for i, t := range data {
		if dlStates[t.State] {
			downloading = append(downloading, i)
		} else if seedStates[t.State] {
			seeding = append(seeding, i)
		} else if strings.Contains(t.State, "paused") {
			paused = append(paused, i)
		}
	}

	if len(downloading) > 0 {
		fmt.Println(ui.Bold("  Active Downloads"))
		for _, i := range downloading {
			t := data[i]
			pct := int(t.Progress * 100)
			speed := float64(t.DLSpeed) / 1048576
			size := ui.FmtSize(t.Size)
			barLen := 15
			filled := barLen * pct / 100
			bar := strings.Repeat("█", filled) + strings.Repeat("░", barLen-filled)
			name := t.Name
			if len(name) > 55 {
				name = name[:55]
			}
			fmt.Printf("  [%s] %s %.1f MB/s  %s %s\n", bar, ui.GoldText(fmt.Sprintf("%d%%", pct)), speed, name, ui.Dim(size))
		}
	} else {
		fmt.Printf("  %s\n", ui.Dim("No active downloads"))
	}

	if len(seeding) > 0 {
		fmt.Printf("\n  %s", ui.Dim(fmt.Sprintf("%d seeding", len(seeding))))
	}
	if len(paused) > 0 {
		fmt.Printf("  %s", ui.Dim(fmt.Sprintf("%d paused", len(paused))))
	}
	fmt.Printf("  %s\n\n", ui.Dim(fmt.Sprintf("%d total", len(data))))
}
