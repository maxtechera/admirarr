package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/maxtechera/admirarr/internal/api"
	"github.com/maxtechera/admirarr/internal/arr"
	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/keys"
	"github.com/maxtechera/admirarr/internal/qbit"
	"github.com/maxtechera/admirarr/internal/sabnzbd"
	"github.com/maxtechera/admirarr/internal/seerr"
	"github.com/maxtechera/admirarr/internal/ui"
	"github.com/maxtechera/admirarr/internal/wire"
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
	if statusLive && !ui.IsJSON() {
		if err := runStatusTUI(); err != nil {
			fmt.Printf("  %s\n", ui.Err(fmt.Sprintf("TUI error: %v", err)))
		}
	} else {
		renderDashboard()
	}
}

// ── Collected data from parallel fetches ──

// arrLibraryData holds library stats for any *arr service.
type arrLibraryData struct {
	Service  string
	Label    string // "Movies", "TV Shows", "Music", "Books"
	Total    int
	OnDisk   int
	Missing  int
	Size     int64
	SubLabel string // "episodes", "tracks", "books" (empty for flat lists)
	SubTotal int
	SubHave  int
	Err      bool
}

type dashData struct {
	mu sync.Mutex

	// Services
	serviceUp map[string]bool
	serviceMs map[string]int64

	// Dynamic *arr library stats (ordered)
	arrLibrary []arrLibraryData

	// Media server counts
	jellyfinCounts *jellyfinItemCounts
	jellyfinErr    bool
	plexCounts     *plexLibraryCounts
	plexErr        bool

	// Health (all *arr services)
	healthItems []statusHealthItem

	// Queues (dynamic, keyed by service)
	arrQueues map[string]statusQueueData

	// Torrents
	torrents    []qbit.Torrent
	torrentsErr bool

	// Usenet (SABnzbd)
	sabnzbdQueue *sabnzbd.QueueResponse
	sabnzbdErr   bool

	// Seerr requests
	seerrRequests []seerr.Request
	seerrTotal    int
	seerrErr      bool

	// Indexers
	prowlarrIndexers []statusIndexer
	indexersErr      bool

	// Activity (all *arr services)
	commands []statusCommandInfo

	// Disk
	diskTotal int64
	diskFree  int64
	diskErr   bool

	// Bazarr subtitles
	bazarrWantedEpisodes int
	bazarrWantedMovies   int
	bazarrErr            bool

	// VPN / Gluetun
	vpnConnected bool
	vpnIP        string
	vpnCountry   string
	vpnErr       bool

	// Tautulli streams
	tautulliStreams int
	tautulliErr     bool
}

type jellyfinItemCounts struct {
	MovieCount   int `json:"MovieCount"`
	SeriesCount  int `json:"SeriesCount"`
	EpisodeCount int `json:"EpisodeCount"`
}
type plexLibraryCounts struct {
	Movies   int
	Shows    int
	Episodes int
}
type statusHealthItem struct {
	Svc     string
	Type    string `json:"type"`
	Message string `json:"message"`
}
type statusQueueData struct {
	Total   int
	Records []arr.QueueRecord
	Err     bool
}
type statusCommandInfo struct {
	Svc    string
	Name   string `json:"name"`
	Status string `json:"status"`
}
type statusIndexer struct {
	Name   string `json:"name"`
	Enable bool   `json:"enable"`
}

// arrLibraryLabel maps service names to human labels.
var arrLibraryLabel = map[string]string{
	"radarr":   "Movies",
	"sonarr":   "TV Shows",
	"lidarr":   "Music",
	"readarr":  "Books",
	"whisparr": "Adult",
}

func renderDashboard() {
	d := &dashData{
		serviceUp: make(map[string]bool),
		serviceMs: make(map[string]int64),
		arrQueues: make(map[string]statusQueueData),
	}

	// Phase 1: Probe all services across candidate hosts in parallel.
	probed := config.ProbeAll()
	var wg sync.WaitGroup
	for name, ss := range probed {
		d.serviceUp[name] = ss.Up
		d.serviceMs[name] = ss.LatencyMs
	}

	// Phase 2: Fetch all data in parallel (only from reachable services)
	fetch := func(fn func()) {
		wg.Add(1)
		go func() { defer wg.Done(); fn() }()
	}

	// Dynamic *arr loop: health, queues, commands, library for ALL *arr services
	for _, name := range config.AllServiceNames() {
		def, ok := config.GetServiceDef(name)
		if !ok || def.APIVer == "" || !d.serviceUp[name] {
			continue
		}
		svcName := name
		client := arr.New(svcName)

		// Health (all *arr)
		fetch(func() {
			items, err := client.Health()
			if err == nil {
				var tagged []statusHealthItem
				for _, item := range items {
					tagged = append(tagged, statusHealthItem{Svc: svcName, Type: item.Type, Message: item.Message})
				}
				d.mu.Lock()
				d.healthItems = append(d.healthItems, tagged...)
				d.mu.Unlock()
			}
		})

		// Queues (all *arr except prowlarr)
		if svcName != "prowlarr" {
			fetch(func() {
				page, err := client.Queue(10)
				d.mu.Lock()
				if err != nil {
					d.arrQueues[svcName] = statusQueueData{Err: true}
				} else {
					d.arrQueues[svcName] = statusQueueData{Total: page.TotalRecords, Records: page.Records}
				}
				d.mu.Unlock()
			})
		}

		// Commands (all *arr except prowlarr)
		if svcName != "prowlarr" {
			fetch(func() {
				cmds, err := client.Commands()
				if err == nil {
					var tagged []statusCommandInfo
					for _, c := range cmds {
						tagged = append(tagged, statusCommandInfo{Svc: svcName, Name: c.Name, Status: c.Status})
					}
					d.mu.Lock()
					d.commands = append(d.commands, tagged...)
					d.mu.Unlock()
				}
			})
		}

		// Library stats (service-specific)
		switch svcName {
		case "radarr", "whisparr":
			fetch(func() {
				movies, err := client.Movies()
				lib := arrLibraryData{Service: svcName, Label: arrLibraryLabel[svcName]}
				if err != nil {
					lib.Err = true
				} else {
					lib.Total = len(movies)
					for _, m := range movies {
						if m.HasFile {
							lib.OnDisk++
						}
						if m.Monitored && !m.HasFile {
							lib.Missing++
						}
						lib.Size += m.SizeOnDisk
					}
				}
				d.mu.Lock()
				d.arrLibrary = append(d.arrLibrary, lib)
				d.mu.Unlock()
			})
		case "sonarr":
			fetch(func() {
				series, err := client.Series()
				lib := arrLibraryData{Service: svcName, Label: "TV Shows", SubLabel: "episodes"}
				if err != nil {
					lib.Err = true
				} else {
					lib.Total = len(series)
					for _, s := range series {
						lib.SubTotal += s.Statistics.EpisodeCount
						lib.SubHave += s.Statistics.EpisodeFileCount
						lib.Size += s.Statistics.SizeOnDisk
					}
				}
				d.mu.Lock()
				d.arrLibrary = append(d.arrLibrary, lib)
				d.mu.Unlock()
			})
		case "lidarr":
			fetch(func() {
				artists, err := client.Artists()
				lib := arrLibraryData{Service: svcName, Label: "Music", SubLabel: "tracks"}
				if err != nil {
					lib.Err = true
				} else {
					lib.Total = len(artists)
					for _, a := range artists {
						lib.SubTotal += a.Statistics.TrackCount
						lib.SubHave += a.Statistics.TrackFileCount
						lib.Size += a.Statistics.SizeOnDisk
					}
				}
				d.mu.Lock()
				d.arrLibrary = append(d.arrLibrary, lib)
				d.mu.Unlock()
			})
		case "readarr":
			fetch(func() {
				authors, err := client.Authors()
				lib := arrLibraryData{Service: svcName, Label: "Books", SubLabel: "books"}
				if err != nil {
					lib.Err = true
				} else {
					lib.Total = len(authors)
					for _, a := range authors {
						lib.SubTotal += a.Statistics.BookCount
						lib.SubHave += a.Statistics.BookFileCount
						lib.Size += a.Statistics.SizeOnDisk
					}
				}
				d.mu.Lock()
				d.arrLibrary = append(d.arrLibrary, lib)
				d.mu.Unlock()
			})
		case "prowlarr":
			fetch(func() {
				indexers, err := client.Indexers()
				if err != nil {
					d.indexersErr = true
				} else {
					var idxs []statusIndexer
					for _, idx := range indexers {
						idxs = append(idxs, statusIndexer{Name: idx.Name, Enable: idx.Enable})
					}
					d.prowlarrIndexers = idxs
				}
			})
		}
	}

	// Jellyfin counts
	if d.serviceUp["jellyfin"] {
		fetch(func() {
			var counts jellyfinItemCounts
			if err := api.GetJSON("jellyfin", "Items/Counts", nil, &counts); err != nil {
				d.jellyfinErr = true
			} else {
				d.jellyfinCounts = &counts
			}
		})
	}

	// Plex library counts
	if d.serviceUp["plex"] {
		fetch(func() {
			counts, err := fetchPlexCounts()
			if err != nil {
				d.plexErr = true
			} else {
				d.plexCounts = counts
			}
		})
	}

	// qBittorrent
	if d.serviceUp["qbittorrent"] {
		fetch(func() {
			torrents, err := qbit.New().Torrents()
			if err != nil {
				d.torrentsErr = true
			} else {
				d.torrents = torrents
			}
		})
	}

	// SABnzbd
	if d.serviceUp["sabnzbd"] {
		fetch(func() {
			q, err := sabnzbd.New().Queue()
			if err != nil {
				d.sabnzbdErr = true
			} else {
				d.sabnzbdQueue = q
			}
		})
	}

	// Seerr
	if d.serviceUp["seerr"] {
		fetch(func() {
			page, err := seerr.New().Requests(8)
			if err != nil {
				d.seerrErr = true
			} else {
				d.seerrRequests = page.Results
				d.seerrTotal = page.PageInfo.Results
			}
		})
	}

	// Bazarr subtitle stats
	if d.serviceUp["bazarr"] {
		fetch(func() {
			var wanted struct {
				Total int `json:"total"`
			}
			if err := api.GetJSON("bazarr", "api/episodes/wanted", map[string]string{"start": "0", "length": "0"}, &wanted); err != nil {
				d.bazarrErr = true
			} else {
				d.bazarrWantedEpisodes = wanted.Total
			}
		})
		fetch(func() {
			var wanted struct {
				Total int `json:"total"`
			}
			if err := api.GetJSON("bazarr", "api/movies/wanted", map[string]string{"start": "0", "length": "0"}, &wanted); err != nil {
				d.bazarrErr = true
			} else {
				d.bazarrWantedMovies = wanted.Total
			}
		})
	}

	// VPN / Gluetun
	if d.serviceUp["gluetun"] || config.IsConfigured("gluetun") {
		fetch(func() {
			vpn := wire.GetVPNStatus()
			if vpn.Err != nil {
				d.vpnErr = true
			} else {
				d.vpnConnected = vpn.Connected
				d.vpnIP = vpn.IP
				d.vpnCountry = vpn.Country
			}
		})
	}

	// Tautulli active streams
	if d.serviceUp["tautulli"] {
		fetch(func() {
			var activity struct {
				Response struct {
					Data struct {
						StreamCount string `json:"stream_count"`
					} `json:"data"`
				} `json:"response"`
			}
			tautulliURL := config.ServiceURL("tautulli")
			key := keys.Get("tautulli")
			if key == "" {
				d.tautulliErr = true
			} else {
				c := &http.Client{Timeout: 5 * time.Second}
				resp, err := c.Get(fmt.Sprintf("%s/api/v2?apikey=%s&cmd=get_activity", tautulliURL, key))
				if err != nil {
					d.tautulliErr = true
				} else {
					defer resp.Body.Close()
					if err := json.NewDecoder(resp.Body).Decode(&activity); err != nil {
						d.tautulliErr = true
					} else {
						fmt.Sscanf(activity.Response.Data.StreamCount, "%d", &d.tautulliStreams)
					}
				}
			}
		})
	}

	// Disk (local, fast)
	fetch(func() {
		total, free, err := getStatfs(config.DataPath())
		if err != nil {
			d.diskErr = true
		} else {
			d.diskTotal = total
			d.diskFree = free
		}
	})

	wg.Wait()

	// Sort arrLibrary into a stable order
	sortArrLibrary(d)

	// ── JSON output ──
	if ui.IsJSON() {
		renderJSON(d, probed)
		return
	}

	// ── CLI Render ──
	now := time.Now().Format("15:04:05")
	fmt.Printf("\n  %s %s %s    %s  %s\n",
		ui.GoldText("⚓"), ui.Bold("ADMIRARR"), ui.Dim("v"+ui.Version),
		ui.Dim("Command your fleet."), ui.Dim(now))
	fmt.Println(ui.Separator())

	// Fleet
	fmt.Printf("\n  %s\n", ui.Bold("Fleet"))
	names := config.AllServiceNames()
	for _, name := range names {
		def, _ := config.GetServiceDef(name)
		if def.Port == 0 {
			continue
		}
		ss := probed[name]
		host := ss.Host
		addr := fmt.Sprintf(":%d", config.ServicePort(name))
		if host != "" && host != "localhost" && host != "127.0.0.1" {
			addr = fmt.Sprintf("%s:%d", host, config.ServicePort(name))
		}
		rtLabel := "  " + ui.Dim(ss.Runtime.Label)
		if ss.Up {
			fmt.Printf("  %s %-13s%s %s%s\n", ui.Ok("●"), name,
				ui.Dim(addr),
				ui.Dim(fmt.Sprintf("%dms", ss.LatencyMs)),
				rtLabel)
		} else {
			fmt.Printf("  %s %-13s%s %s%s\n", ui.Err("○"), name,
				ui.Dim(addr),
				ui.Err("down"),
				rtLabel)
		}
		for _, w := range ss.Warnings {
			fmt.Printf("  %s %s %s\n", ui.Warn("⚠"), name, ui.Warn(w))
		}
	}

	// Indexers
	fmt.Printf("\n  %s\n", ui.Bold("Indexers"))
	if !d.indexersErr && d.serviceUp["prowlarr"] {
		configuredNames := make(map[string]bool)
		enabled, disabled := 0, 0
		for _, idx := range d.prowlarrIndexers {
			configuredNames[strings.ToLower(idx.Name)] = true
			if idx.Enable {
				enabled++
			} else {
				disabled++
			}
		}
		var missing []string
		for _, rec := range recommendedIndexers {
			if !configuredNames[strings.ToLower(rec.Name)] {
				missing = append(missing, rec.Name)
			}
		}
		have := len(recommendedIndexers) - len(missing)
		total := len(recommendedIndexers)
		if len(missing) == 0 {
			fmt.Printf("  %s %d/%d recommended\n", ui.Ok("✓"), have, total)
		} else {
			fmt.Printf("  %s %d/%d recommended — missing: %s\n",
				ui.Warn("⚠"), have, total, strings.Join(missing, ", "))
		}
		summary := fmt.Sprintf("%d configured, %d enabled", len(d.prowlarrIndexers), enabled)
		if disabled > 0 {
			summary += fmt.Sprintf(", %d disabled", disabled)
		}
		fmt.Printf("  %s\n", ui.Dim(summary))
	} else {
		fmt.Printf("  %s\n", ui.Dim("unavailable"))
	}

	// Library
	fmt.Printf("\n  %s\n", ui.Bold("Library"))

	// Media server counts
	if d.jellyfinCounts != nil {
		fmt.Printf("  %s  %d movies, %d series, %d episodes\n",
			ui.GoldText("Jellyfin"), d.jellyfinCounts.MovieCount, d.jellyfinCounts.SeriesCount, d.jellyfinCounts.EpisodeCount)
	}
	if d.plexCounts != nil {
		parts := []string{}
		if d.plexCounts.Movies > 0 {
			parts = append(parts, fmt.Sprintf("%d movies", d.plexCounts.Movies))
		}
		if d.plexCounts.Shows > 0 {
			parts = append(parts, fmt.Sprintf("%d shows", d.plexCounts.Shows))
		}
		if d.plexCounts.Episodes > 0 {
			parts = append(parts, fmt.Sprintf("%d episodes", d.plexCounts.Episodes))
		}
		if len(parts) > 0 {
			fmt.Printf("  %s      %s\n", ui.GoldText("Plex"), strings.Join(parts, ", "))
		}
	}

	// Dynamic *arr library
	for _, lib := range d.arrLibrary {
		if lib.Err {
			fmt.Printf("  %-10s %s\n", ui.GoldText(lib.Label), ui.Dim("unavailable"))
			continue
		}
		if lib.SubLabel != "" {
			// Series-like: shows + episodes, artists + tracks, authors + books
			fmt.Printf("  %-10s %d total, %s  %s\n",
				ui.GoldText(lib.Label),
				lib.Total,
				ui.Ok(fmt.Sprintf("%d/%d %s", lib.SubHave, lib.SubTotal, lib.SubLabel)),
				ui.Dim(ui.FmtSize(lib.Size)))
		} else {
			// Movie-like: total, on disk, missing
			missStr := ui.Ok("0 missing")
			if lib.Missing > 0 {
				missStr = ui.Err(fmt.Sprintf("%d missing", lib.Missing))
			}
			fmt.Printf("  %-10s %d total, %s, %s  %s\n",
				ui.GoldText(lib.Label),
				lib.Total,
				ui.Ok(fmt.Sprintf("%d on disk", lib.OnDisk)),
				missStr,
				ui.Dim(ui.FmtSize(lib.Size)))
		}
	}

	// Subtitles (Bazarr)
	if d.serviceUp["bazarr"] && !d.bazarrErr {
		wanted := d.bazarrWantedEpisodes + d.bazarrWantedMovies
		if wanted > 0 {
			fmt.Printf("  %s %s  %s\n",
				ui.Warn("Subs"),
				ui.Warn(fmt.Sprintf("%d missing", wanted)),
				ui.Dim(fmt.Sprintf("(%d episodes, %d movies)", d.bazarrWantedEpisodes, d.bazarrWantedMovies)))
		} else {
			fmt.Printf("  %-10s %s\n", ui.GoldText("Subs"), ui.Ok("all synced"))
		}
	}

	// Active streams (Tautulli)
	if d.serviceUp["tautulli"] && !d.tautulliErr {
		if d.tautulliStreams > 0 {
			fmt.Printf("  %-10s %s\n", ui.GoldText("Streams"), ui.Ok(fmt.Sprintf("%d active", d.tautulliStreams)))
		} else {
			fmt.Printf("  %-10s %s\n", ui.GoldText("Streams"), ui.Dim("none"))
		}
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
			case 4:
				icon = "●"
				colorFn = ui.Ok
			case 2:
				icon = "◐"
				colorFn = ui.Warn
			case 1:
				icon = "○"
				colorFn = ui.GoldText
			}
			if len(title) > 42 {
				title = title[:42] + "…"
			}
			s4k := ""
			if r.Is4K {
				s4k = " 4K"
			}
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
				if c.Status == "queued" {
					icon = ui.Dim("◷")
				}
				fmt.Printf("  %s %s %s %s\n", icon, ui.Dim("["+c.Svc+"]"), c.Name, ui.Dim(c.Status))
			}
		}
	}

	// Health
	if len(d.healthItems) > 0 {
		fmt.Printf("\n  %s\n", ui.Bold("Health"))
		for _, item := range d.healthItems {
			level := ui.Warn("WARN")
			if item.Type == "error" {
				level = ui.Err("ERROR")
			}
			msg := item.Message
			if len(msg) > 60 {
				msg = msg[:60] + "…"
			}
			fmt.Printf("  %s %s %s\n", level, ui.Dim("["+item.Svc+"]"), msg)
		}
	}

	// Queues (dynamic)
	hasQueue := false
	queueOrder := []string{"radarr", "sonarr", "lidarr", "readarr", "whisparr"}
	for _, svc := range queueOrder {
		if q, ok := d.arrQueues[svc]; ok && q.Total > 0 {
			hasQueue = true
			break
		}
	}
	if hasQueue {
		fmt.Printf("\n  %s\n", ui.Bold("Queues"))
		for _, svc := range queueOrder {
			q, ok := d.arrQueues[svc]
			if !ok || q.Total == 0 {
				continue
			}
			for _, rec := range q.Records {
				colorFn := ui.Err
				switch rec.TrackedDownloadState {
				case "downloading":
					colorFn = ui.Ok
				case "importPending":
					colorFn = ui.Warn
				}
				title := rec.Title
				if len(title) > 45 {
					title = title[:45] + "…"
				}
				pct := ""
				if rec.Size > 0 {
					p := (1 - rec.Sizeleft/rec.Size) * 100
					pct = ui.Dim(fmt.Sprintf(" %.0f%%", p))
				}
				fmt.Printf("  %s %-14s %s%s\n", ui.Dim("["+svc+"]"), colorFn(rec.TrackedDownloadState), title, pct)
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
			if dlStates[t.State] {
				dlCount++
				totalDL += t.DLSpeed
			}
			if seedStates[t.State] {
				seedCount++
			}
		}
		fmt.Printf("\n  %s\n", ui.Bold("Torrents"))
		if dlCount > 0 {
			for _, t := range d.torrents {
				if !dlStates[t.State] {
					continue
				}
				pct := int(t.Progress * 100)
				speed := float64(t.DLSpeed) / 1048576
				name := t.Name
				if len(name) > 40 {
					name = name[:40] + "…"
				}
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

	// Usenet (SABnzbd)
	if d.sabnzbdQueue != nil {
		fmt.Printf("\n  %s\n", ui.Bold("Usenet"))
		if len(d.sabnzbdQueue.Slots) > 0 {
			for _, slot := range d.sabnzbdQueue.Slots {
				name := slot.Filename
				if len(name) > 45 {
					name = name[:45] + "…"
				}
				fmt.Printf("  %s %-14s %s  %s\n",
					ui.Dim("[sabnzbd]"),
					ui.Ok(slot.Status),
					name,
					ui.Dim(slot.Percentage+"%"))
			}
		}
		pauseStr := ""
		if d.sabnzbdQueue.Paused {
			pauseStr = " " + ui.Warn("(paused)")
		}
		fmt.Printf("  %s\n", ui.Dim(fmt.Sprintf(
			"%d in queue, %s remaining, %s/s%s",
			d.sabnzbdQueue.NoOfSlots, d.sabnzbdQueue.SizeLeft, d.sabnzbdQueue.Speed, pauseStr)))
	}

	// VPN
	if !d.vpnErr && (d.serviceUp["gluetun"] || config.IsConfigured("gluetun")) {
		fmt.Printf("\n  %s  ", ui.Bold("VPN"))
		if d.vpnConnected {
			detail := ui.Ok("connected")
			if d.vpnIP != "" {
				detail += "  " + ui.Dim(d.vpnIP)
			}
			if d.vpnCountry != "" {
				detail += " " + ui.Dim("("+d.vpnCountry+")")
			}
			fmt.Println(detail)
		} else {
			fmt.Println(ui.Err("disconnected"))
		}
	}

	// Disk
	if !d.diskErr {
		used := d.diskTotal - d.diskFree
		pct := float64(used) / float64(d.diskTotal) * 100
		barLen := 20
		filled := int(float64(barLen) * pct / 100)
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barLen-filled)
		colorFn := ui.Ok
		if pct >= 90 {
			colorFn = ui.Err
		} else if pct >= 80 {
			colorFn = ui.Warn
		}
		fmt.Printf("\n  %s  [%s] %s  %s free / %s\n",
			ui.Bold("Disk"), bar, colorFn(fmt.Sprintf("%.0f%%", pct)),
			ui.FmtSize(d.diskFree), ui.FmtSize(d.diskTotal))
	}
	fmt.Println()
}

// sortArrLibrary orders library entries: radarr, sonarr, lidarr, readarr, whisparr.
func sortArrLibrary(d *dashData) {
	order := map[string]int{"radarr": 0, "sonarr": 1, "lidarr": 2, "readarr": 3, "whisparr": 4}
	sorted := make([]arrLibraryData, len(d.arrLibrary))
	copy(sorted, d.arrLibrary)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if order[sorted[i].Service] > order[sorted[j].Service] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	d.arrLibrary = sorted
}

// fetchPlexCounts gets library section counts from Plex.
// Uses JSON parsing since addAuth sets Accept: application/json for Plex.
func fetchPlexCounts() (*plexLibraryCounts, error) {
	var sections struct {
		MediaContainer struct {
			Directory []struct {
				Key   string `json:"key"`
				Type  string `json:"type"`
				Title string `json:"title"`
			} `json:"Directory"`
		} `json:"MediaContainer"`
	}
	if err := api.GetJSON("plex", "library/sections", nil, &sections); err != nil {
		return nil, err
	}

	counts := &plexLibraryCounts{}
	for _, dir := range sections.MediaContainer.Directory {
		// Get item count per section without downloading items
		endpoint := fmt.Sprintf("library/sections/%s/all", dir.Key)
		var container struct {
			MediaContainer struct {
				TotalSize int `json:"totalSize"`
			} `json:"MediaContainer"`
		}
		params := map[string]string{
			"X-Plex-Container-Start": "0",
			"X-Plex-Container-Size":  "0",
		}
		if err := api.GetJSON("plex", endpoint, params, &container); err != nil {
			continue
		}
		switch dir.Type {
		case "movie":
			counts.Movies += container.MediaContainer.TotalSize
		case "show":
			counts.Shows += container.MediaContainer.TotalSize
		}
	}
	return counts, nil
}

// ── JSON output ──

func renderJSON(d *dashData, probed map[string]config.ServiceStatus) {
	type serviceJSON struct {
		Name     string   `json:"name"`
		Host     string   `json:"host"`
		Up       bool     `json:"up"`
		Ms       int64    `json:"latency_ms"`
		Runtime  string   `json:"runtime"`
		Warnings []string `json:"warnings,omitempty"`
	}
	type arrLibJSON struct {
		Service  string `json:"service"`
		Label    string `json:"label"`
		Total    int    `json:"total"`
		OnDisk   int    `json:"on_disk,omitempty"`
		Missing  int    `json:"missing,omitempty"`
		Size     int64  `json:"size"`
		SubLabel string `json:"sub_label,omitempty"`
		SubTotal int    `json:"sub_total,omitempty"`
		SubHave  int    `json:"sub_on_disk,omitempty"`
	}
	type mediaServerJSON struct {
		Name     string `json:"name"`
		Movies   int    `json:"movies"`
		Shows    int    `json:"shows"`
		Episodes int    `json:"episodes,omitempty"`
	}
	type requestJSON struct {
		Status int    `json:"status"`
		Is4K   bool   `json:"is_4k"`
		User   string `json:"user"`
	}
	type queueItemJSON struct {
		Title string  `json:"title"`
		State string  `json:"state"`
		Size  float64 `json:"size"`
	}
	type torrentJSON struct {
		Name     string  `json:"name"`
		Size     int64   `json:"size"`
		Progress float64 `json:"progress"`
		DLSpeed  int64   `json:"dl_speed"`
		State    string  `json:"state"`
	}
	type usenetSlotJSON struct {
		Filename   string `json:"filename"`
		Status     string `json:"status"`
		Percentage string `json:"percentage"`
		Size       string `json:"size"`
	}
	type usenetJSON struct {
		Speed  string           `json:"speed"`
		Slots  int              `json:"slots"`
		Paused bool             `json:"paused"`
		Items  []usenetSlotJSON `json:"items"`
	}
	type subtitlesJSON struct {
		WantedEpisodes int `json:"wanted_episodes"`
		WantedMovies   int `json:"wanted_movies"`
	}
	type vpnJSON struct {
		Connected bool   `json:"connected"`
		IP        string `json:"ip,omitempty"`
		Country   string `json:"country,omitempty"`
	}
	type diskJSON struct {
		Total int64 `json:"total"`
		Free  int64 `json:"free"`
	}
	type indexerJSON struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	type indexersSummaryJSON struct {
		Configured  int           `json:"configured"`
		Enabled     int           `json:"enabled"`
		Recommended int           `json:"recommended"`
		Missing     []string      `json:"missing"`
		Indexers    []indexerJSON `json:"indexers"`
	}
	type statusOut struct {
		Services     []serviceJSON              `json:"services"`
		MediaServers []mediaServerJSON          `json:"media_servers,omitempty"`
		Library      []arrLibJSON               `json:"library"`
		Subtitles    *subtitlesJSON             `json:"subtitles,omitempty"`
		Streams      *int                       `json:"streams,omitempty"`
		Health       []statusHealthItem         `json:"health"`
		Indexers     *indexersSummaryJSON       `json:"indexers,omitempty"`
		Requests     []requestJSON              `json:"requests"`
		Queues       map[string][]queueItemJSON `json:"queues"`
		Torrents     []torrentJSON              `json:"torrents"`
		Usenet       *usenetJSON                `json:"usenet,omitempty"`
		VPN          *vpnJSON                   `json:"vpn,omitempty"`
		Disk         *diskJSON                  `json:"disk,omitempty"`
	}

	out := statusOut{
		Queues: make(map[string][]queueItemJSON),
	}

	// Services
	for _, name := range config.AllServiceNames() {
		def, _ := config.GetServiceDef(name)
		if def.Port == 0 {
			continue
		}
		ss := probed[name]
		out.Services = append(out.Services, serviceJSON{Name: name, Host: ss.Host, Up: ss.Up, Ms: ss.LatencyMs, Runtime: ss.Runtime.Label, Warnings: ss.Warnings})
	}

	// Media servers
	if d.jellyfinCounts != nil {
		out.MediaServers = append(out.MediaServers, mediaServerJSON{
			Name: "jellyfin", Movies: d.jellyfinCounts.MovieCount,
			Shows: d.jellyfinCounts.SeriesCount, Episodes: d.jellyfinCounts.EpisodeCount,
		})
	}
	if d.plexCounts != nil {
		out.MediaServers = append(out.MediaServers, mediaServerJSON{
			Name: "plex", Movies: d.plexCounts.Movies, Shows: d.plexCounts.Shows,
		})
	}

	// Library
	for _, lib := range d.arrLibrary {
		if lib.Err {
			continue
		}
		out.Library = append(out.Library, arrLibJSON{
			Service: lib.Service, Label: lib.Label, Total: lib.Total,
			OnDisk: lib.OnDisk, Missing: lib.Missing, Size: lib.Size,
			SubLabel: lib.SubLabel, SubTotal: lib.SubTotal, SubHave: lib.SubHave,
		})
	}
	if out.Library == nil {
		out.Library = []arrLibJSON{}
	}

	// Subtitles
	if d.serviceUp["bazarr"] && !d.bazarrErr {
		out.Subtitles = &subtitlesJSON{WantedEpisodes: d.bazarrWantedEpisodes, WantedMovies: d.bazarrWantedMovies}
	}

	// Streams
	if d.serviceUp["tautulli"] && !d.tautulliErr {
		out.Streams = &d.tautulliStreams
	}

	// Health
	out.Health = d.healthItems
	if out.Health == nil {
		out.Health = []statusHealthItem{}
	}

	// Requests
	for _, r := range d.seerrRequests {
		out.Requests = append(out.Requests, requestJSON{Status: r.Status, Is4K: r.Is4K, User: r.RequestedBy.DisplayName})
	}
	if out.Requests == nil {
		out.Requests = []requestJSON{}
	}

	// Queues (dynamic)
	for svc, q := range d.arrQueues {
		var items []queueItemJSON
		for _, rec := range q.Records {
			items = append(items, queueItemJSON{Title: rec.Title, State: rec.TrackedDownloadState, Size: rec.Size})
		}
		if items == nil {
			items = []queueItemJSON{}
		}
		out.Queues[svc] = items
	}

	// Torrents
	for _, t := range d.torrents {
		out.Torrents = append(out.Torrents, torrentJSON{Name: t.Name, Size: t.Size, Progress: t.Progress, DLSpeed: t.DLSpeed, State: t.State})
	}
	if out.Torrents == nil {
		out.Torrents = []torrentJSON{}
	}

	// Usenet
	if d.sabnzbdQueue != nil {
		u := &usenetJSON{
			Speed:  d.sabnzbdQueue.Speed,
			Slots:  d.sabnzbdQueue.NoOfSlots,
			Paused: d.sabnzbdQueue.Paused,
		}
		for _, slot := range d.sabnzbdQueue.Slots {
			u.Items = append(u.Items, usenetSlotJSON{
				Filename: slot.Filename, Status: slot.Status,
				Percentage: slot.Percentage, Size: slot.Size,
			})
		}
		if u.Items == nil {
			u.Items = []usenetSlotJSON{}
		}
		out.Usenet = u
	}

	// VPN
	if !d.vpnErr && (d.serviceUp["gluetun"] || config.IsConfigured("gluetun")) {
		out.VPN = &vpnJSON{Connected: d.vpnConnected, IP: d.vpnIP, Country: d.vpnCountry}
	}

	// Disk
	if !d.diskErr {
		out.Disk = &diskJSON{Total: d.diskTotal, Free: d.diskFree}
	}

	// Indexers
	if !d.indexersErr && d.serviceUp["prowlarr"] {
		idxSummary := &indexersSummaryJSON{
			Recommended: len(recommendedIndexers),
		}
		configuredNames := make(map[string]bool)
		for _, idx := range d.prowlarrIndexers {
			idxSummary.Indexers = append(idxSummary.Indexers, indexerJSON{Name: idx.Name, Enabled: idx.Enable})
			configuredNames[strings.ToLower(idx.Name)] = true
			if idx.Enable {
				idxSummary.Enabled++
			}
		}
		idxSummary.Configured = len(d.prowlarrIndexers)
		for _, rec := range recommendedIndexers {
			if !configuredNames[strings.ToLower(rec.Name)] {
				idxSummary.Missing = append(idxSummary.Missing, rec.Name)
			}
		}
		if idxSummary.Missing == nil {
			idxSummary.Missing = []string{}
		}
		if idxSummary.Indexers == nil {
			idxSummary.Indexers = []indexerJSON{}
		}
		out.Indexers = idxSummary
	}

	ui.PrintJSON(out)
}

func fmtETA(secs int64) string {
	if secs <= 0 || secs > 8640000 {
		return ""
	}
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	if secs < 3600 {
		return fmt.Sprintf("%dm%ds", secs/60, secs%60)
	}
	return fmt.Sprintf("%dh%dm", secs/3600, (secs%3600)/60)
}
