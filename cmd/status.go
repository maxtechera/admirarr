package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/maxtechera/admirarr/internal/api"
	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/keys"
	"github.com/maxtechera/admirarr/internal/ui"
	"github.com/spf13/cobra"
)

var statusLive bool
var statusInterval int

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Dashboard: services, library, queues, requests, disk",
	Long: `Fleet dashboard — full stack status at a glance.

Shows services, library stats, Seerr requests, *Arr queues, active tasks,
qBittorrent downloads, and disk usage. All API calls run in parallel for speed.

Use --live for auto-refreshing mode.`,
	Example: "  admirarr status\n  admirarr status --live\n  admirarr status --live --interval 10",
	Run:     runStatus,
}

func init() {
	statusCmd.Flags().BoolVar(&statusLive, "live", false, "Auto-refresh the dashboard")
	statusCmd.Flags().IntVar(&statusInterval, "interval", 5, "Refresh interval in seconds (with --live)")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) {
	if statusLive {
		if err := runStatusTUI(); err != nil {
			fmt.Printf("  %s\n", ui.Err(fmt.Sprintf("TUI error: %v", err)))
		}
	} else {
		renderDashboard()
	}
}

// ── Collected data from parallel fetches ──

type dashData struct {
	mu sync.Mutex

	// Services
	serviceUp  map[string]bool
	serviceMs  map[string]int64

	// Library
	movies       []movieInfo
	moviesErr    bool
	series       []seriesInfo
	seriesErr    bool

	// Health
	healthItems []healthItem

	// Queues
	radarrQueue  queueData
	sonarrQueue  queueData

	// Torrents
	torrents    []torrentInfo
	torrentsErr bool

	// Seerr requests
	seerrRequests []seerrRequest
	seerrTotal    int
	seerrErr      bool

	// Activity
	commands []commandInfo

	// Disk
	diskTotal int64
	diskFree  int64
	diskErr   bool
}

type movieInfo struct {
	HasFile    bool  `json:"hasFile"`
	Monitored  bool  `json:"monitored"`
	SizeOnDisk int64 `json:"sizeOnDisk"`
}
type seriesInfo struct {
	Stats struct {
		EpisodeCount     int   `json:"episodeCount"`
		EpisodeFileCount int   `json:"episodeFileCount"`
		SizeOnDisk       int64 `json:"sizeOnDisk"`
	} `json:"statistics"`
}
type healthItem struct {
	Svc     string
	Type    string `json:"type"`
	Message string `json:"message"`
}
type queueData struct {
	Total   int
	Records []queueRecord
	Err     bool
}
type queueRecord struct {
	Title    string  `json:"title"`
	State    string  `json:"trackedDownloadState"`
	Sizeleft float64 `json:"sizeleft"`
	Size     float64 `json:"size"`
}
type torrentInfo struct {
	Name     string  `json:"name"`
	Size     int64   `json:"size"`
	Progress float64 `json:"progress"`
	DLSpeed  int64   `json:"dlspeed"`
	State    string  `json:"state"`
	ETA      int64   `json:"eta"`
}
type seerrRequest struct {
	Status int  `json:"status"`
	Is4K   bool `json:"is4k"`
	Media  struct {
		MediaType string `json:"mediaType"`
		TmdbID    int    `json:"tmdbId"`
	} `json:"media"`
	RequestedBy struct {
		DisplayName string `json:"displayName"`
	} `json:"requestedBy"`
}
type commandInfo struct {
	Svc    string
	Name   string `json:"name"`
	Status string `json:"status"`
}

func renderDashboard() {
	d := &dashData{
		serviceUp: make(map[string]bool),
		serviceMs: make(map[string]int64),
	}

	// Phase 1: Check all services in parallel
	var wg sync.WaitGroup
	for _, name := range config.AllServiceNames() {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			t0 := time.Now()
			up := api.CheckReachable(n)
			ms := time.Since(t0).Milliseconds()
			d.mu.Lock()
			d.serviceUp[n] = up
			d.serviceMs[n] = ms
			d.mu.Unlock()
		}(name)
	}
	wg.Wait()

	// Phase 2: Fetch all data in parallel (only from reachable services)
	fetch := func(fn func()) {
		wg.Add(1)
		go func() { defer wg.Done(); fn() }()
	}

	if d.serviceUp["radarr"] {
		fetch(func() {
			if err := api.GetJSON("radarr", "api/v3/movie", nil, &d.movies); err != nil {
				d.moviesErr = true
			}
		})
		fetch(func() {
			var raw struct {
				TotalRecords int `json:"totalRecords"`
				Records      []queueRecord `json:"records"`
			}
			if err := api.GetJSON("radarr", "api/v3/queue", map[string]string{"pageSize": "10"}, &raw); err != nil {
				d.radarrQueue.Err = true
			} else {
				d.radarrQueue.Total = raw.TotalRecords
				d.radarrQueue.Records = raw.Records
			}
		})
		fetch(func() {
			var items []healthItem
			if api.GetJSON("radarr", "api/v3/health", nil, &items) == nil {
				for i := range items {
					items[i].Svc = "radarr"
				}
				d.mu.Lock()
				d.healthItems = append(d.healthItems, items...)
				d.mu.Unlock()
			}
		})
		fetch(func() {
			var cmds []commandInfo
			if api.GetJSON("radarr", "api/v3/command", nil, &cmds) == nil {
				for i := range cmds {
					cmds[i].Svc = "radarr"
				}
				d.mu.Lock()
				d.commands = append(d.commands, cmds...)
				d.mu.Unlock()
			}
		})
	}

	if d.serviceUp["sonarr"] {
		fetch(func() {
			if err := api.GetJSON("sonarr", "api/v3/series", nil, &d.series); err != nil {
				d.seriesErr = true
			}
		})
		fetch(func() {
			var raw struct {
				TotalRecords int `json:"totalRecords"`
				Records      []queueRecord `json:"records"`
			}
			if err := api.GetJSON("sonarr", "api/v3/queue", map[string]string{"pageSize": "10"}, &raw); err != nil {
				d.sonarrQueue.Err = true
			} else {
				d.sonarrQueue.Total = raw.TotalRecords
				d.sonarrQueue.Records = raw.Records
			}
		})
		fetch(func() {
			var items []healthItem
			if api.GetJSON("sonarr", "api/v3/health", nil, &items) == nil {
				for i := range items {
					items[i].Svc = "sonarr"
				}
				d.mu.Lock()
				d.healthItems = append(d.healthItems, items...)
				d.mu.Unlock()
			}
		})
		fetch(func() {
			var cmds []commandInfo
			if api.GetJSON("sonarr", "api/v3/command", nil, &cmds) == nil {
				for i := range cmds {
					cmds[i].Svc = "sonarr"
				}
				d.mu.Lock()
				d.commands = append(d.commands, cmds...)
				d.mu.Unlock()
			}
		})
	}

	if d.serviceUp["prowlarr"] {
		fetch(func() {
			var items []healthItem
			if api.GetJSON("prowlarr", "api/v1/health", nil, &items) == nil {
				for i := range items {
					items[i].Svc = "prowlarr"
				}
				d.mu.Lock()
				d.healthItems = append(d.healthItems, items...)
				d.mu.Unlock()
			}
		})
	}

	if d.serviceUp["qbittorrent"] {
		fetch(func() {
			url := fmt.Sprintf("%s/api/v2/torrents/info", config.ServiceURL("qbittorrent"))
			c := &http.Client{Timeout: 3 * time.Second}
			resp, err := c.Get(url)
			if err != nil {
				d.torrentsErr = true
				return
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			if json.Unmarshal(body, &d.torrents) != nil {
				d.torrentsErr = true
			}
		})
	}

	if d.serviceUp["seerr"] {
		fetch(func() {
			var raw struct {
				PageInfo struct {
					Results int `json:"results"`
				} `json:"pageInfo"`
				Results []seerrRequest `json:"results"`
			}
			if api.GetJSON("seerr", "api/v1/request", map[string]string{"take": "8", "sort": "added"}, &raw) == nil {
				d.seerrRequests = raw.Results
				d.seerrTotal = raw.PageInfo.Results
			} else {
				d.seerrErr = true
			}
		})
	}

	// Disk (local, fast)
	fetch(func() {
		total, free, err := getStatfs(config.MediaPathWSL())
		if err != nil {
			d.diskErr = true
		} else {
			d.diskTotal = total
			d.diskFree = free
		}
	})

	wg.Wait()

	// ── Render ──
	now := time.Now().Format("15:04:05")
	fmt.Printf("\n  %s %s %s    %s  %s\n",
		ui.GoldText("⚓"), ui.Bold("ADMIRARR"), ui.Dim("v"+ui.Version),
		ui.Dim("Command your fleet."), ui.Dim(now))
	fmt.Println(ui.Separator())

	// Services (compact 2-column)
	fmt.Printf("\n  %s\n", ui.Bold("Fleet"))
	for _, name := range config.AllServiceNames() {
		svc := config.Get().Services[name]
		if d.serviceUp[name] {
			fmt.Printf("  %s %-13s%s %s\n", ui.Ok("●"), name,
				ui.Dim(fmt.Sprintf(":%d", svc.Port)),
				ui.Dim(fmt.Sprintf("%dms", d.serviceMs[name])))
		} else {
			fmt.Printf("  %s %-13s%s %s\n", ui.Err("○"), name,
				ui.Dim(fmt.Sprintf(":%d", svc.Port)),
				ui.Err("down"))
		}
	}

	// Library
	fmt.Printf("\n  %s\n", ui.Bold("Library"))
	if !d.moviesErr && d.movies != nil {
		have, missing := 0, 0
		var totalSize int64
		for _, m := range d.movies {
			if m.HasFile { have++ }
			if m.Monitored && !m.HasFile { missing++ }
			totalSize += m.SizeOnDisk
		}
		missStr := ui.Ok("0 missing")
		if missing > 0 { missStr = ui.Err(fmt.Sprintf("%d missing", missing)) }
		fmt.Printf("  %s     %d total, %s, %s  %s\n",
			ui.GoldText("Movies"), len(d.movies), ui.Ok(fmt.Sprintf("%d on disk", have)), missStr, ui.Dim(ui.FmtSize(totalSize)))
	} else {
		fmt.Printf("  %s     %s\n", ui.GoldText("Movies"), ui.Dim("unavailable"))
	}
	if !d.seriesErr && d.series != nil {
		totalEps, haveEps := 0, 0
		var totalSize int64
		for _, s := range d.series {
			totalEps += s.Stats.EpisodeCount
			haveEps += s.Stats.EpisodeFileCount
			totalSize += s.Stats.SizeOnDisk
		}
		fmt.Printf("  %s   %d shows, %s  %s\n",
			ui.GoldText("TV Shows"), len(d.series), ui.Ok(fmt.Sprintf("%d/%d episodes", haveEps, totalEps)), ui.Dim(ui.FmtSize(totalSize)))
	} else {
		fmt.Printf("  %s   %s\n", ui.GoldText("TV Shows"), ui.Dim("unavailable"))
	}

	// Requests
	fmt.Printf("\n  %s", ui.Bold("Requests"))
	if d.seerrTotal > 0 {
		fmt.Printf(" %s\n", ui.Dim(fmt.Sprintf("(%d)", d.seerrTotal)))
	} else {
		fmt.Println()
	}
	if !d.seerrErr && len(d.seerrRequests) > 0 {
		for _, r := range d.seerrRequests {
			title, year := resolveTitle(r.Media.MediaType, r.Media.TmdbID)
			status := statusNames[r.Status]
			icon, colorFn := "○", ui.Dim
			switch r.Status {
			case 4: icon = "●"; colorFn = ui.Ok
			case 2: icon = "◐"; colorFn = ui.Warn
			case 1: icon = "○"; colorFn = ui.GoldText
			}
			if len(title) > 42 { title = title[:42] + "…" }
			s4k := ""
			if r.Is4K { s4k = " 4K" }
			fmt.Printf("  %s %-12s %s (%s)%s\n", colorFn(icon), colorFn(status), title, year, s4k)
		}
	} else if !d.seerrErr {
		fmt.Printf("  %s\n", ui.Dim("No requests"))
	} else {
		fmt.Printf("  %s\n", ui.Dim("Seerr unavailable"))
	}

	// Activity
	activeCount := 0
	for _, c := range d.commands {
		if c.Status == "started" || c.Status == "queued" {
			activeCount++
		}
	}
	if activeCount > 0 {
		fmt.Printf("\n  %s\n", ui.Bold("Activity"))
		for _, c := range d.commands {
			if c.Status == "started" || c.Status == "queued" {
				icon := ui.Warn("⟳")
				if c.Status == "queued" { icon = ui.Dim("◷") }
				fmt.Printf("  %s %s %s %s\n", icon, ui.Dim("["+c.Svc+"]"), c.Name, ui.Dim(c.Status))
			}
		}
	}

	// Health
	if len(d.healthItems) > 0 {
		fmt.Printf("\n  %s\n", ui.Bold("Health"))
		for _, item := range d.healthItems {
			level := ui.Warn("WARN")
			if item.Type == "error" { level = ui.Err("ERROR") }
			msg := item.Message
			if len(msg) > 60 { msg = msg[:60] + "…" }
			fmt.Printf("  %s %s %s\n", level, ui.Dim("["+item.Svc+"]"), msg)
		}
	}

	// Queues
	hasQueue := d.radarrQueue.Total > 0 || d.sonarrQueue.Total > 0
	if hasQueue {
		fmt.Printf("\n  %s\n", ui.Bold("Queues"))
		for _, q := range []struct{ name string; data queueData }{
			{"radarr", d.radarrQueue},
			{"sonarr", d.sonarrQueue},
		} {
			for _, rec := range q.data.Records {
				colorFn := ui.Err
				if rec.State == "downloading" { colorFn = ui.Ok } else if rec.State == "importPending" { colorFn = ui.Warn }
				title := rec.Title
				if len(title) > 45 { title = title[:45] + "…" }
				pct := ""
				if rec.Size > 0 {
					p := (1 - rec.Sizeleft/rec.Size) * 100
					pct = ui.Dim(fmt.Sprintf(" %.0f%%", p))
				}
				fmt.Printf("  %s %-14s %s%s\n", ui.Dim("["+q.name+"]"), colorFn(rec.State), title, pct)
			}
		}
	}

	// Torrents
	if d.torrents != nil {
		dlStates := map[string]bool{"downloading": true, "stalledDL": true, "forcedDL": true, "metaDL": true}
		seedStates := map[string]bool{"uploading": true, "stalledUP": true, "forcedUP": true}
		var dlCount, seedCount int
		var totalDL int64
		for _, t := range d.torrents {
			if dlStates[t.State] { dlCount++; totalDL += t.DLSpeed }
			if seedStates[t.State] { seedCount++ }
		}
		fmt.Printf("\n  %s\n", ui.Bold("Torrents"))
		if dlCount > 0 {
			for _, t := range d.torrents {
				if !dlStates[t.State] { continue }
				pct := int(t.Progress * 100)
				speed := float64(t.DLSpeed) / 1048576
				name := t.Name
				if len(name) > 40 { name = name[:40] + "…" }
				barLen := 12
				filled := barLen * pct / 100
				bar := strings.Repeat("█", filled) + strings.Repeat("░", barLen-filled)
				eta := fmtETA(t.ETA)
				fmt.Printf("  [%s] %s %s  %s %s\n",
					bar, ui.GoldText(fmt.Sprintf("%3d%%", pct)),
					fmt.Sprintf("%.1f MB/s", speed), name, ui.Dim(eta))
			}
		}
		fmt.Printf("  %s\n", ui.Dim(fmt.Sprintf(
			"%d downloading (%.1f MB/s), %d seeding, %d total",
			dlCount, float64(totalDL)/1048576, seedCount, len(d.torrents))))
	}

	// Disk
	if !d.diskErr {
		used := d.diskTotal - d.diskFree
		pct := float64(used) / float64(d.diskTotal) * 100
		barLen := 20
		filled := int(float64(barLen) * pct / 100)
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barLen-filled)
		colorFn := ui.Ok
		if pct >= 90 { colorFn = ui.Err } else if pct >= 80 { colorFn = ui.Warn }
		fmt.Printf("\n  %s  [%s] %s  %s free / %s\n",
			ui.Bold("Disk"), bar, colorFn(fmt.Sprintf("%.0f%%", pct)),
			ui.FmtSize(d.diskFree), ui.FmtSize(d.diskTotal))
	}
	fmt.Println()
}

func fmtETA(secs int64) string {
	if secs <= 0 || secs > 8640000 { return "" }
	if secs < 60 { return fmt.Sprintf("%ds", secs) }
	if secs < 3600 { return fmt.Sprintf("%dm%ds", secs/60, secs%60) }
	return fmt.Sprintf("%dh%dm", secs/3600, (secs%3600)/60)
}

// keep keys import used
var _ = keys.Get
