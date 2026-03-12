package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/maxtechera/admirarr/internal/api"
	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/ui"
)

// ── Messages ──

type tickMsg time.Time

type serviceResult struct {
	Name string
	Up   bool
	Ms   int64
}

type moviesResult struct {
	Movies []movieInfo
	Err    bool
}

type seriesResult struct {
	Series []seriesInfo
	Err    bool
}

type healthResult struct {
	Items []healthItem
}

type queueResult struct {
	Svc     string
	Total   int
	Records []queueRecord
}

type torrentsResult struct {
	Torrents []torrentInfo
	Err      bool
}

type seerrResult struct {
	Requests []seerrRequest
	Total    int
	Titles   map[int]titleInfo // tmdbID -> title
	Err      bool
}

type titleInfo struct {
	Title string
	Year  string
}

type commandsResult struct {
	Commands []commandInfo
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
	services      map[string]serviceResult
	movies        *moviesResult
	series        *seriesResult
	health        *healthResult
	radarrQueue   *queueResult
	sonarrQueue   *queueResult
	torrents      *torrentsResult
	seerr         *seerrResult
	commands      *commandsResult
	disk          *diskResult

	// State
	loading    bool
	lastUpdate time.Time
	tick       int
	quitting   bool
}

func newTuiModel() tuiModel {
	return tuiModel{
		services: make(map[string]serviceResult),
		loading:  true,
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
			wg.Add(1)
			go func(n string) {
				defer wg.Done()
				t0 := time.Now()
				up := api.CheckReachable(n)
				ms := time.Since(t0).Milliseconds()
				addMsg(serviceResult{Name: n, Up: up, Ms: ms})
			}(name)
		}

		// Movies
		wg.Add(1)
		go func() {
			defer wg.Done()
			var movies []movieInfo
			err := api.GetJSON("radarr", "api/v3/movie", nil, &movies)
			addMsg(moviesResult{Movies: movies, Err: err != nil})
		}()

		// Series
		wg.Add(1)
		go func() {
			defer wg.Done()
			var series []seriesInfo
			err := api.GetJSON("sonarr", "api/v3/series", nil, &series)
			addMsg(seriesResult{Series: series, Err: err != nil})
		}()

		// Health
		wg.Add(1)
		go func() {
			defer wg.Done()
			var items []healthItem
			for _, svc := range []string{"radarr", "sonarr", "prowlarr"} {
				ver := config.ServiceAPIVer(svc)
				var h []healthItem
				if api.GetJSON(svc, fmt.Sprintf("api/%s/health", ver), nil, &h) == nil {
					for i := range h {
						h[i].Svc = svc
					}
					items = append(items, h...)
				}
			}
			addMsg(healthResult{Items: items})
		}()

		// Queues
		for _, svc := range []string{"radarr", "sonarr"} {
			wg.Add(1)
			go func(s string) {
				defer wg.Done()
				ver := config.ServiceAPIVer(s)
				var raw struct {
					TotalRecords int           `json:"totalRecords"`
					Records      []queueRecord `json:"records"`
				}
				api.GetJSON(s, fmt.Sprintf("api/%s/queue", ver), map[string]string{"pageSize": "10"}, &raw)
				addMsg(queueResult{Svc: s, Total: raw.TotalRecords, Records: raw.Records})
			}(svc)
		}

		// Torrents
		wg.Add(1)
		go func() {
			defer wg.Done()
			url := fmt.Sprintf("%s/api/v2/torrents/info", config.ServiceURL("qbittorrent"))
			c := &http.Client{Timeout: 3 * time.Second}
			resp, err := c.Get(url)
			if err != nil {
				addMsg(torrentsResult{Err: true})
				return
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			var t []torrentInfo
			if json.Unmarshal(body, &t) != nil {
				addMsg(torrentsResult{Err: true})
				return
			}
			addMsg(torrentsResult{Torrents: t})
		}()

		// Seerr
		wg.Add(1)
		go func() {
			defer wg.Done()
			var raw struct {
				PageInfo struct {
					Results int `json:"results"`
				} `json:"pageInfo"`
				Results []seerrRequest `json:"results"`
			}
			if api.GetJSON("seerr", "api/v1/request", map[string]string{"take": "8", "sort": "added"}, &raw) != nil {
				addMsg(seerrResult{Err: true})
				return
			}
			// Resolve titles in parallel
			titles := make(map[int]titleInfo)
			var tmu sync.Mutex
			var twg sync.WaitGroup
			for _, r := range raw.Results {
				twg.Add(1)
				go func(mediaType string, tmdbID int) {
					defer twg.Done()
					t, y := resolveTitle(mediaType, tmdbID)
					tmu.Lock()
					titles[tmdbID] = titleInfo{Title: t, Year: y}
					tmu.Unlock()
				}(r.Media.MediaType, r.Media.TmdbID)
			}
			twg.Wait()
			addMsg(seerrResult{Requests: raw.Results, Total: raw.PageInfo.Results, Titles: titles})
		}()

		// Commands
		wg.Add(1)
		go func() {
			defer wg.Done()
			var all []commandInfo
			for _, svc := range []string{"radarr", "sonarr"} {
				var cmds []commandInfo
				if api.GetJSON(svc, fmt.Sprintf("api/%s/command", config.ServiceAPIVer(svc)), nil, &cmds) == nil {
					for i := range cmds {
						cmds[i].Svc = svc
					}
					all = append(all, cmds...)
				}
			}
			addMsg(commandsResult{Commands: all})
		}()

		// Disk
		wg.Add(1)
		go func() {
			defer wg.Done()
			total, free, err := getStatfs(config.MediaPathWSL())
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
		return m, tea.Batch(cmds...)

	case serviceResult:
		m.services[msg.Name] = msg
	case moviesResult:
		m.movies = &msg
	case seriesResult:
		m.series = &msg
	case healthResult:
		m.health = &msg
	case queueResult:
		if msg.Svc == "radarr" {
			m.radarrQueue = &msg
		} else {
			m.sonarrQueue = &msg
		}
	case torrentsResult:
		m.torrents = &msg
	case seerrResult:
		m.seerr = &msg
	case commandsResult:
		m.commands = &msg
	case diskResult:
		m.disk = &msg
	}

	return m, nil
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
	// Render 2 columns
	for i := 0; i < len(names); i += 2 {
		left := renderServiceCell(m.services, names[i])
		right := ""
		if i+1 < len(names) {
			right = renderServiceCell(m.services, names[i+1])
		}
		b.WriteString(fmt.Sprintf("  %-38s%s\n", left, right))
	}

	// Library
	b.WriteString(fmt.Sprintf("\n  %s\n", ui.Bold("Library")))
	if m.movies != nil && !m.movies.Err {
		have, missing := 0, 0
		var sz int64
		for _, mv := range m.movies.Movies {
			if mv.HasFile { have++ }
			if mv.Monitored && !mv.HasFile { missing++ }
			sz += mv.SizeOnDisk
		}
		missStr := ui.Ok("0 missing")
		if missing > 0 { missStr = ui.Err(fmt.Sprintf("%d missing", missing)) }
		b.WriteString(fmt.Sprintf("  %s     %d total, %s, %s  %s\n",
			ui.GoldText("Movies"), len(m.movies.Movies), ui.Ok(fmt.Sprintf("%d on disk", have)), missStr, ui.Dim(ui.FmtSize(sz))))
	} else {
		b.WriteString(fmt.Sprintf("  %s     %s\n", ui.GoldText("Movies"), ui.Dim("…")))
	}
	if m.series != nil && !m.series.Err {
		te, he := 0, 0
		var sz int64
		for _, s := range m.series.Series {
			te += s.Stats.EpisodeCount
			he += s.Stats.EpisodeFileCount
			sz += s.Stats.SizeOnDisk
		}
		b.WriteString(fmt.Sprintf("  %s   %d shows, %s  %s\n",
			ui.GoldText("TV Shows"), len(m.series.Series), ui.Ok(fmt.Sprintf("%d/%d episodes", he, te)), ui.Dim(ui.FmtSize(sz))))
	} else {
		b.WriteString(fmt.Sprintf("  %s   %s\n", ui.GoldText("TV Shows"), ui.Dim("…")))
	}

	// Requests
	if m.seerr != nil && !m.seerr.Err && len(m.seerr.Requests) > 0 {
		b.WriteString(fmt.Sprintf("\n  %s %s\n", ui.Bold("Requests"), ui.Dim(fmt.Sprintf("(%d)", m.seerr.Total))))
		for _, r := range m.seerr.Requests {
			ti := m.seerr.Titles[r.Media.TmdbID]
			title := ti.Title
			if title == "" { title = "…" }
			if len(title) > 40 { title = title[:40] + "…" }
			icon, colorFn := "○", ui.Dim
			switch r.Status {
			case 4: icon = "●"; colorFn = ui.Ok
			case 2: icon = "◐"; colorFn = ui.Warn
			case 1: icon = "○"; colorFn = ui.GoldText
			}
			status := statusNames[r.Status]
			s4k := ""
			if r.Is4K { s4k = " 4K" }
			b.WriteString(fmt.Sprintf("  %s %-12s %s (%s)%s\n", colorFn(icon), colorFn(status), title, ti.Year, s4k))
		}
	}

	// Activity
	if m.commands != nil {
		var active []commandInfo
		for _, c := range m.commands.Commands {
			if c.Status == "started" || c.Status == "queued" {
				active = append(active, c)
			}
		}
		if len(active) > 0 {
			b.WriteString(fmt.Sprintf("\n  %s\n", ui.Bold("Activity")))
			for _, c := range active {
				icon := ui.Warn("⟳")
				if c.Status == "queued" { icon = ui.Dim("◷") }
				b.WriteString(fmt.Sprintf("  %s %s %s %s\n", icon, ui.Dim("["+c.Svc+"]"), c.Name, ui.Dim(c.Status)))
			}
		}
	}

	// Health
	if m.health != nil && len(m.health.Items) > 0 {
		b.WriteString(fmt.Sprintf("\n  %s\n", ui.Bold("Health")))
		for _, item := range m.health.Items {
			level := ui.Warn("WARN")
			if item.Type == "error" { level = ui.Err("ERROR") }
			msg := item.Message
			if len(msg) > 58 { msg = msg[:58] + "…" }
			b.WriteString(fmt.Sprintf("  %s %s %s\n", level, ui.Dim("["+item.Svc+"]"), msg))
		}
	}

	// Queues
	hasQ := (m.radarrQueue != nil && m.radarrQueue.Total > 0) || (m.sonarrQueue != nil && m.sonarrQueue.Total > 0)
	if hasQ {
		b.WriteString(fmt.Sprintf("\n  %s\n", ui.Bold("Queues")))
		for _, q := range []*queueResult{m.radarrQueue, m.sonarrQueue} {
			if q == nil { continue }
			for _, rec := range q.Records {
				colorFn := ui.Err
				if rec.State == "downloading" { colorFn = ui.Ok } else if rec.State == "importPending" { colorFn = ui.Warn }
				title := rec.Title
				if len(title) > 42 { title = title[:42] + "…" }
				pct := ""
				if rec.Size > 0 { pct = ui.Dim(fmt.Sprintf(" %.0f%%", (1-rec.Sizeleft/rec.Size)*100)) }
				b.WriteString(fmt.Sprintf("  %s %-14s %s%s\n", ui.Dim("["+q.Svc+"]"), colorFn(rec.State), title, pct))
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
			if dlStates[t.State] { dlCount++; totalDL += t.DLSpeed }
			if seedStates[t.State] { seedCount++ }
		}
		b.WriteString(fmt.Sprintf("\n  %s\n", ui.Bold("Torrents")))
		if dlCount > 0 {
			for _, t := range m.torrents.Torrents {
				if !dlStates[t.State] { continue }
				pct := int(t.Progress * 100)
				speed := float64(t.DLSpeed) / 1048576
				name := t.Name
				if len(name) > 38 { name = name[:38] + "…" }
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

	// Disk
	if m.disk != nil && !m.disk.Err {
		used := m.disk.Total - m.disk.Free
		pct := float64(used) / float64(m.disk.Total) * 100
		barLen := 20
		filled := int(float64(barLen) * pct / 100)
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barLen-filled)
		colorFn := ui.Ok
		if pct >= 90 { colorFn = ui.Err } else if pct >= 80 { colorFn = ui.Warn }
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
	// Force lipgloss to use color (alt screen)
	lipgloss.SetHasDarkBackground(true)
	p := tea.NewProgram(newTuiModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
