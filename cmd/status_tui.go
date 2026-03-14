package cmd

import (
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/maxtechera/admirarr/internal/api"
	"github.com/maxtechera/admirarr/internal/arr"
	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/qbit"
	"github.com/maxtechera/admirarr/internal/sabnzbd"
	"github.com/maxtechera/admirarr/internal/seerr"
	"github.com/maxtechera/admirarr/internal/ui"
)

// ── Messages ──

type tickMsg time.Time

type serviceResult struct {
	Name string
	Up   bool
	Ms   int64
}

type arrLibraryResult struct {
	Data arrLibraryData
}

type jellyfinCountsResult struct {
	Counts *jellyfinItemCounts
	Err    bool
}

type plexCountsResult struct {
	Counts *plexLibraryCounts
	Err    bool
}

type healthResult struct {
	Items []statusHealthItem
}

type queueResult struct {
	Svc     string
	Total   int
	Records []arr.QueueRecord
}

type torrentsResult struct {
	Torrents []qbit.Torrent
	Err      bool
}

type sabnzbdResult struct {
	Queue *sabnzbd.QueueResponse
	Err   bool
}

type seerrResult struct {
	Requests []seerr.Request
	Total    int
	Titles   map[int]titleInfo // tmdbID -> title
	Err      bool
}

type titleInfo struct {
	Title string
	Year  string
}

type commandsResult struct {
	Commands []statusCommandInfo
}

type indexersResult struct {
	Indexers []statusIndexer
	Err      bool
}

type diskResult struct {
	Total int64
	Free  int64
	Err   bool
}

// ── Model ──

type tuiModel struct {
	width  int
	height int

	// Data
	services       map[string]serviceResult
	arrLibrary     []arrLibraryData
	jellyfinCounts *jellyfinCountsResult
	plexCounts     *plexCountsResult
	health         *healthResult
	arrQueues      map[string]*queueResult
	torrents       *torrentsResult
	sabnzbdData    *sabnzbdResult
	seerr          *seerrResult
	commands       *commandsResult
	indexers       *indexersResult
	disk           *diskResult

	// State
	loading    bool
	lastUpdate time.Time
	tick       int
	quitting   bool
}

func newTuiModel() tuiModel {
	return tuiModel{
		services:  make(map[string]serviceResult),
		arrQueues: make(map[string]*queueResult),
		loading:   true,
	}
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		fetchAll(),
		tickCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// fetchAll fires all API calls concurrently, returns results as messages.
func fetchAll() tea.Cmd {
	return func() tea.Msg {
		var msgs []tea.Msg
		var mu sync.Mutex
		var wg sync.WaitGroup

		addMsg := func(m tea.Msg) {
			mu.Lock()
			msgs = append(msgs, m)
			mu.Unlock()
		}

		// Services
		for _, name := range config.AllServiceNames() {
			def, _ := config.GetServiceDef(name)
			if def.Port == 0 {
				continue
			}
			wg.Add(1)
			go func(n string) {
				defer wg.Done()
				t0 := time.Now()
				up := api.CheckReachable(n)
				ms := time.Since(t0).Milliseconds()
				addMsg(serviceResult{Name: n, Up: up, Ms: ms})
			}(name)
		}

		// Dynamic *arr loop
		for _, name := range config.AllServiceNames() {
			def, ok := config.GetServiceDef(name)
			if !ok || def.APIVer == "" {
				continue
			}
			svcName := name
			client := arr.New(svcName)

			// Health
			wg.Add(1)
			go func() {
				defer wg.Done()
				items, err := client.Health()
				if err != nil {
					return
				}
				var tagged []statusHealthItem
				for _, item := range items {
					tagged = append(tagged, statusHealthItem{Svc: svcName, Type: item.Type, Message: item.Message})
				}
				addMsg(healthResult{Items: tagged})
			}()

			// Queues + Commands (all except prowlarr)
			if svcName != "prowlarr" {
				wg.Add(1)
				go func() {
					defer wg.Done()
					page, _ := client.Queue(10)
					if page != nil {
						addMsg(queueResult{Svc: svcName, Total: page.TotalRecords, Records: page.Records})
					} else {
						addMsg(queueResult{Svc: svcName})
					}
				}()

				wg.Add(1)
				go func() {
					defer wg.Done()
					cmds, err := client.Commands()
					if err == nil {
						var tagged []statusCommandInfo
						for _, c := range cmds {
							tagged = append(tagged, statusCommandInfo{Svc: svcName, Name: c.Name, Status: c.Status})
						}
						addMsg(commandsResult{Commands: tagged})
					}
				}()
			}

			// Library stats
			switch svcName {
			case "radarr", "whisparr":
				wg.Add(1)
				go func() {
					defer wg.Done()
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
					addMsg(arrLibraryResult{Data: lib})
				}()
			case "sonarr":
				wg.Add(1)
				go func() {
					defer wg.Done()
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
					addMsg(arrLibraryResult{Data: lib})
				}()
			case "lidarr":
				wg.Add(1)
				go func() {
					defer wg.Done()
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
					addMsg(arrLibraryResult{Data: lib})
				}()
			case "readarr":
				wg.Add(1)
				go func() {
					defer wg.Done()
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
					addMsg(arrLibraryResult{Data: lib})
				}()
			case "prowlarr":
				wg.Add(1)
				go func() {
					defer wg.Done()
					indexers, err := client.Indexers()
					var idxs []statusIndexer
					if err == nil {
						for _, idx := range indexers {
							idxs = append(idxs, statusIndexer{Name: idx.Name, Enable: idx.Enable})
						}
					}
					addMsg(indexersResult{Indexers: idxs, Err: err != nil})
				}()
			}
		}

		// Jellyfin counts
		wg.Add(1)
		go func() {
			defer wg.Done()
			var counts jellyfinItemCounts
			err := api.GetJSON("jellyfin", "Items/Counts", nil, &counts)
			addMsg(jellyfinCountsResult{Counts: &counts, Err: err != nil})
		}()

		// Plex counts
		wg.Add(1)
		go func() {
			defer wg.Done()
			counts, err := fetchPlexCounts()
			addMsg(plexCountsResult{Counts: counts, Err: err != nil})
		}()

		// Torrents
		wg.Add(1)
		go func() {
			defer wg.Done()
			torrents, err := qbit.New().Torrents()
			addMsg(torrentsResult{Torrents: torrents, Err: err != nil})
		}()

		// SABnzbd
		wg.Add(1)
		go func() {
			defer wg.Done()
			q, err := sabnzbd.New().Queue()
			addMsg(sabnzbdResult{Queue: q, Err: err != nil})
		}()

		// Seerr
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := seerr.New()
			page, err := client.Requests(8)
			if err != nil {
				addMsg(seerrResult{Err: true})
				return
			}
			titles := make(map[int]titleInfo)
			var tmu sync.Mutex
			var twg sync.WaitGroup
			for _, r := range page.Results {
				twg.Add(1)
				go func(mediaType string, tmdbID int) {
					defer twg.Done()
					t, y := client.ResolveTitle(mediaType, tmdbID)
					tmu.Lock()
					titles[tmdbID] = titleInfo{Title: t, Year: y}
					tmu.Unlock()
				}(r.Media.MediaType, r.Media.TmdbID)
			}
			twg.Wait()
			addMsg(seerrResult{Requests: page.Results, Total: page.PageInfo.Results, Titles: titles})
		}()

		// Disk
		wg.Add(1)
		go func() {
			defer wg.Done()
			total, free, err := getStatfs(config.DataPath())
			addMsg(diskResult{Total: total, Free: free, Err: err != nil})
		}()

		wg.Wait()
		return batchResults(msgs)
	}
}

type batchResults []tea.Msg

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "r":
			m.loading = true
			return m, fetchAll()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		m.tick++
		m.loading = true
		return m, tea.Batch(fetchAll(), tickCmd())

	case batchResults:
		var cmds []tea.Cmd
		for _, sub := range msg {
			var cmd tea.Cmd
			updated, cmd := m.Update(sub)
			m = updated.(tuiModel)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		m.loading = false
		m.lastUpdate = time.Now()
		// Sort library after all results are in
		sortTuiLibrary(&m)
		return m, tea.Batch(cmds...)

	case serviceResult:
		m.services[msg.Name] = msg
	case arrLibraryResult:
		// Replace existing entry for this service or append
		found := false
		for i, lib := range m.arrLibrary {
			if lib.Service == msg.Data.Service {
				m.arrLibrary[i] = msg.Data
				found = true
				break
			}
		}
		if !found {
			m.arrLibrary = append(m.arrLibrary, msg.Data)
		}
	case jellyfinCountsResult:
		m.jellyfinCounts = &msg
	case plexCountsResult:
		m.plexCounts = &msg
	case healthResult:
		if m.health == nil {
			m.health = &healthResult{}
		}
		m.health.Items = append(m.health.Items, msg.Items...)
	case queueResult:
		m.arrQueues[msg.Svc] = &msg
	case torrentsResult:
		m.torrents = &msg
	case sabnzbdResult:
		m.sabnzbdData = &msg
	case seerrResult:
		m.seerr = &msg
	case commandsResult:
		if m.commands == nil {
			m.commands = &commandsResult{}
		}
		m.commands.Commands = append(m.commands.Commands, msg.Commands...)
	case indexersResult:
		m.indexers = &msg
	case diskResult:
		m.disk = &msg
	}

	return m, nil
}

func sortTuiLibrary(m *tuiModel) {
	order := map[string]int{"radarr": 0, "sonarr": 1, "lidarr": 2, "readarr": 3, "whisparr": 4}
	for i := 0; i < len(m.arrLibrary); i++ {
		for j := i + 1; j < len(m.arrLibrary); j++ {
			if order[m.arrLibrary[i].Service] > order[m.arrLibrary[j].Service] {
				m.arrLibrary[i], m.arrLibrary[j] = m.arrLibrary[j], m.arrLibrary[i]
			}
		}
	}
}

func (m tuiModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Header
	spinner := "●"
	if m.loading {
		frames := []string{"◐", "◓", "◑", "◒"}
		spinner = ui.GoldText(frames[m.tick%4])
	} else {
		spinner = ui.Ok("●")
	}
	ts := m.lastUpdate.Format("15:04:05")
	if m.lastUpdate.IsZero() {
		ts = "loading…"
	}

	b.WriteString(fmt.Sprintf("\n  %s %s %s    %s  %s  %s\n",
		ui.GoldText("⚓"), ui.Bold("ADMIRARR"), ui.Dim("v"+ui.Version),
		ui.Dim("Command your fleet."), ui.Dim(ts), spinner))
	b.WriteString(ui.Separator() + "\n")

	// Fleet
	b.WriteString(fmt.Sprintf("\n  %s\n", ui.Bold("Fleet")))
	names := config.AllServiceNames()
	var displayNames []string
	for _, n := range names {
		def, _ := config.GetServiceDef(n)
		if def.Port > 0 {
			displayNames = append(displayNames, n)
		}
	}
	for i := 0; i < len(displayNames); i += 2 {
		left := renderServiceCell(m.services, displayNames[i])
		right := ""
		if i+1 < len(displayNames) {
			right = renderServiceCell(m.services, displayNames[i+1])
		}
		b.WriteString(fmt.Sprintf("  %-38s%s\n", left, right))
	}

	// Indexers
	b.WriteString(fmt.Sprintf("\n  %s\n", ui.Bold("Indexers")))
	if m.indexers != nil && !m.indexers.Err {
		configuredNames := make(map[string]bool)
		enabled, disabled := 0, 0
		for _, idx := range m.indexers.Indexers {
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
			b.WriteString(fmt.Sprintf("  %s %d/%d recommended\n", ui.Ok("✓"), have, total))
		} else {
			b.WriteString(fmt.Sprintf("  %s %d/%d recommended — missing: %s\n",
				ui.Warn("⚠"), have, total, strings.Join(missing, ", ")))
		}
		summary := fmt.Sprintf("%d configured, %d enabled", len(m.indexers.Indexers), enabled)
		if disabled > 0 {
			summary += fmt.Sprintf(", %d disabled", disabled)
		}
		b.WriteString(fmt.Sprintf("  %s\n", ui.Dim(summary)))
	} else {
		b.WriteString(fmt.Sprintf("  %s\n", ui.Dim("…")))
	}

	// Library
	b.WriteString(fmt.Sprintf("\n  %s\n", ui.Bold("Library")))

	// Media server counts
	if m.jellyfinCounts != nil && !m.jellyfinCounts.Err {
		c := m.jellyfinCounts.Counts
		b.WriteString(fmt.Sprintf("  %s  %d movies, %d series, %d episodes\n",
			ui.GoldText("Jellyfin"), c.MovieCount, c.SeriesCount, c.EpisodeCount))
	}
	if m.plexCounts != nil && !m.plexCounts.Err && m.plexCounts.Counts != nil {
		pc := m.plexCounts.Counts
		var parts []string
		if pc.Movies > 0 {
			parts = append(parts, fmt.Sprintf("%d movies", pc.Movies))
		}
		if pc.Shows > 0 {
			parts = append(parts, fmt.Sprintf("%d shows", pc.Shows))
		}
		if len(parts) > 0 {
			b.WriteString(fmt.Sprintf("  %s      %s\n", ui.GoldText("Plex"), strings.Join(parts, ", ")))
		}
	}

	// Dynamic *arr library
	for _, lib := range m.arrLibrary {
		if lib.Err {
			b.WriteString(fmt.Sprintf("  %-10s %s\n", ui.GoldText(lib.Label), ui.Dim("…")))
			continue
		}
		if lib.SubLabel != "" {
			b.WriteString(fmt.Sprintf("  %-10s %d total, %s  %s\n",
				ui.GoldText(lib.Label), lib.Total,
				ui.Ok(fmt.Sprintf("%d/%d %s", lib.SubHave, lib.SubTotal, lib.SubLabel)),
				ui.Dim(ui.FmtSize(lib.Size))))
		} else {
			missStr := ui.Ok("0 missing")
			if lib.Missing > 0 {
				missStr = ui.Err(fmt.Sprintf("%d missing", lib.Missing))
			}
			b.WriteString(fmt.Sprintf("  %-10s %d total, %s, %s  %s\n",
				ui.GoldText(lib.Label), lib.Total,
				ui.Ok(fmt.Sprintf("%d on disk", lib.OnDisk)), missStr,
				ui.Dim(ui.FmtSize(lib.Size))))
		}
	}

	// Requests
	if m.seerr != nil && !m.seerr.Err && len(m.seerr.Requests) > 0 {
		b.WriteString(fmt.Sprintf("\n  %s %s\n", ui.Bold("Requests"), ui.Dim(fmt.Sprintf("(%d)", m.seerr.Total))))
		for _, r := range m.seerr.Requests {
			ti := m.seerr.Titles[r.Media.TmdbID]
			title := ti.Title
			if title == "" {
				title = "…"
			}
			if len(title) > 40 {
				title = title[:40] + "…"
			}
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
			status := statusNames[r.Status]
			s4k := ""
			if r.Is4K {
				s4k = " 4K"
			}
			b.WriteString(fmt.Sprintf("  %s %-12s %s (%s)%s\n", colorFn(icon), colorFn(status), title, ti.Year, s4k))
		}
	}

	// Activity
	if m.commands != nil {
		var active []statusCommandInfo
		for _, c := range m.commands.Commands {
			if c.Status == "started" || c.Status == "queued" {
				active = append(active, c)
			}
		}
		if len(active) > 0 {
			b.WriteString(fmt.Sprintf("\n  %s\n", ui.Bold("Activity")))
			for _, c := range active {
				icon := ui.Warn("⟳")
				if c.Status == "queued" {
					icon = ui.Dim("◷")
				}
				b.WriteString(fmt.Sprintf("  %s %s %s %s\n", icon, ui.Dim("["+c.Svc+"]"), c.Name, ui.Dim(c.Status)))
			}
		}
	}

	// Health
	if m.health != nil && len(m.health.Items) > 0 {
		b.WriteString(fmt.Sprintf("\n  %s\n", ui.Bold("Health")))
		for _, item := range m.health.Items {
			level := ui.Warn("WARN")
			if item.Type == "error" {
				level = ui.Err("ERROR")
			}
			msg := item.Message
			if len(msg) > 58 {
				msg = msg[:58] + "…"
			}
			b.WriteString(fmt.Sprintf("  %s %s %s\n", level, ui.Dim("["+item.Svc+"]"), msg))
		}
	}

	// Queues (dynamic)
	queueOrder := []string{"radarr", "sonarr", "lidarr", "readarr", "whisparr"}
	hasQ := false
	for _, svc := range queueOrder {
		if q, ok := m.arrQueues[svc]; ok && q.Total > 0 {
			hasQ = true
			break
		}
	}
	if hasQ {
		b.WriteString(fmt.Sprintf("\n  %s\n", ui.Bold("Queues")))
		for _, svc := range queueOrder {
			q, ok := m.arrQueues[svc]
			if !ok || q.Total == 0 {
				continue
			}
			for _, rec := range q.Records {
				colorFn := ui.Err
				if rec.TrackedDownloadState == "downloading" {
					colorFn = ui.Ok
				} else if rec.TrackedDownloadState == "importPending" {
					colorFn = ui.Warn
				}
				title := rec.Title
				if len(title) > 42 {
					title = title[:42] + "…"
				}
				pct := ""
				if rec.Size > 0 {
					pct = ui.Dim(fmt.Sprintf(" %.0f%%", (1-rec.Sizeleft/rec.Size)*100))
				}
				b.WriteString(fmt.Sprintf("  %s %-14s %s%s\n", ui.Dim("["+svc+"]"), colorFn(rec.TrackedDownloadState), title, pct))
			}
		}
	}

	// Torrents
	if m.torrents != nil && !m.torrents.Err {
		dlStates := map[string]bool{"downloading": true, "stalledDL": true, "forcedDL": true, "metaDL": true}
		seedStates := map[string]bool{"uploading": true, "stalledUP": true, "forcedUP": true}
		var dlCount, seedCount int
		var totalDL int64
		for _, t := range m.torrents.Torrents {
			if dlStates[t.State] {
				dlCount++
				totalDL += t.DLSpeed
			}
			if seedStates[t.State] {
				seedCount++
			}
		}
		b.WriteString(fmt.Sprintf("\n  %s\n", ui.Bold("Torrents")))
		if dlCount > 0 {
			for _, t := range m.torrents.Torrents {
				if !dlStates[t.State] {
					continue
				}
				pct := int(t.Progress * 100)
				speed := float64(t.DLSpeed) / 1048576
				name := t.Name
				if len(name) > 38 {
					name = name[:38] + "…"
				}
				barLen := 12
				filled := barLen * pct / 100
				bar := strings.Repeat("█", filled) + strings.Repeat("░", barLen-filled)
				eta := fmtETA(t.ETA)
				b.WriteString(fmt.Sprintf("  [%s] %s %s  %s %s\n",
					bar, ui.GoldText(fmt.Sprintf("%3d%%", pct)),
					fmt.Sprintf("%.1f MB/s", speed), name, ui.Dim(eta)))
			}
		}
		b.WriteString(fmt.Sprintf("  %s\n", ui.Dim(fmt.Sprintf(
			"%d downloading (%.1f MB/s), %d seeding, %d total",
			dlCount, float64(totalDL)/1048576, seedCount, len(m.torrents.Torrents)))))
	}

	// Usenet (SABnzbd)
	if m.sabnzbdData != nil && !m.sabnzbdData.Err && m.sabnzbdData.Queue != nil {
		q := m.sabnzbdData.Queue
		b.WriteString(fmt.Sprintf("\n  %s\n", ui.Bold("Usenet")))
		for _, slot := range q.Slots {
			name := slot.Filename
			if len(name) > 42 {
				name = name[:42] + "…"
			}
			b.WriteString(fmt.Sprintf("  %s %-14s %s  %s\n",
				ui.Dim("[sabnzbd]"), ui.Ok(slot.Status), name, ui.Dim(slot.Percentage+"%")))
		}
		pauseStr := ""
		if q.Paused {
			pauseStr = " " + ui.Warn("(paused)")
		}
		b.WriteString(fmt.Sprintf("  %s\n", ui.Dim(fmt.Sprintf(
			"%d in queue, %s remaining, %s/s%s",
			q.NoOfSlots, q.SizeLeft, q.Speed, pauseStr))))
	}

	// Disk
	if m.disk != nil && !m.disk.Err {
		used := m.disk.Total - m.disk.Free
		pct := float64(used) / float64(m.disk.Total) * 100
		barLen := 20
		filled := int(float64(barLen) * pct / 100)
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barLen-filled)
		colorFn := ui.Ok
		if pct >= 90 {
			colorFn = ui.Err
		} else if pct >= 80 {
			colorFn = ui.Warn
		}
		b.WriteString(fmt.Sprintf("\n  %s  [%s] %s  %s free / %s\n",
			ui.Bold("Disk"), bar, colorFn(fmt.Sprintf("%.0f%%", pct)),
			ui.FmtSize(m.disk.Free), ui.FmtSize(m.disk.Total)))
	}

	// Footer
	b.WriteString(fmt.Sprintf("\n  %s\n", ui.Dim("r refresh  q quit")))

	return b.String()
}

func renderServiceCell(services map[string]serviceResult, name string) string {
	svc := config.Get().Services[name]
	r, ok := services[name]
	if !ok {
		return fmt.Sprintf("%s %-13s%s %s",
			ui.Dim("◌"), name, ui.Dim(fmt.Sprintf(":%d", svc.Port)), ui.Dim("…"))
	}
	if r.Up {
		return fmt.Sprintf("%s %-13s%s %s",
			ui.Ok("●"), name, ui.Dim(fmt.Sprintf(":%d", svc.Port)),
			ui.Dim(fmt.Sprintf("%dms", r.Ms)))
	}
	return fmt.Sprintf("%s %-13s%s %s",
		ui.Err("○"), name, ui.Dim(fmt.Sprintf(":%d", svc.Port)),
		ui.Err("down"))
}

func runStatusTUI() error {
	lipgloss.SetHasDarkBackground(true)
	p := tea.NewProgram(newTuiModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
