package setup

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/maxtechera/admirarr/internal/api"
	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/ui"
)

// downloadClient represents a *Arr download client entry.
type downloadClient struct {
	ID             int                    `json:"id"`
	Name           string                 `json:"name"`
	Enable         bool                   `json:"enable"`
	Implementation string                 `json:"implementation"`
	ConfigContract string                 `json:"configContract"`
	Fields         []downloadClientField  `json:"fields"`
	Protocol       string                 `json:"protocol"`
	Priority       int                    `json:"priority"`
	Tags           []int                  `json:"tags"`
	RemoveCompletedDownloads bool         `json:"removeCompletedDownloads"`
	RemoveFailedDownloads    bool         `json:"removeFailedDownloads"`
}

type downloadClientField struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

func (dc *downloadClient) getField(name string) interface{} {
	for _, f := range dc.Fields {
		if f.Name == name {
			return f.Value
		}
	}
	return nil
}

func (dc *downloadClient) setField(name string, value interface{}) {
	for i, f := range dc.Fields {
		if f.Name == name {
			dc.Fields[i].Value = value
			return
		}
	}
	dc.Fields = append(dc.Fields, downloadClientField{Name: name, Value: value})
}

// qBitCategory represents a qBittorrent category entry.
type qBitCategory struct {
	SavePath string `json:"save_path"`
}

// ConfigureDownloadClients runs Phase 4: download client + qBittorrent configuration.
func ConfigureDownloadClients(state *SetupState) StepResult {
	r := StepResult{Name: "Download Client Config"}

	fmt.Println(ui.Bold("\n  Phase 4 — Download Client Configuration"))
	fmt.Println(ui.Separator())

	// 4a. Check/fix Radarr download client
	if svc := state.Services["radarr"]; svc != nil && svc.Reachable {
		checkArrDownloadClient(state, &r, "radarr", "movies")
	} else {
		fmt.Printf("  %s %-15s %s\n", ui.Dim("—"), "radarr", ui.Dim("skipped (not reachable)"))
		r.skip()
	}

	// 4a. Check/fix Sonarr download client
	if svc := state.Services["sonarr"]; svc != nil && svc.Reachable {
		checkArrDownloadClient(state, &r, "sonarr", "tv-sonarr")
	} else {
		fmt.Printf("  %s %-15s %s\n", ui.Dim("—"), "sonarr", ui.Dim("skipped (not reachable)"))
		r.skip()
	}

	// 4b. Fix qBittorrent categories
	if svc := state.Services["qbittorrent"]; svc != nil && svc.Reachable {
		checkQbitCategories(state, &r)
		// 4c. Verify default save path
		checkQbitDefaultPath(state, &r)
	} else {
		fmt.Printf("  %s %-15s %s\n", ui.Dim("—"), "qbittorrent", ui.Dim("skipped (not reachable)"))
		r.skip()
	}

	// 4d. Create missing download directories
	checkDownloadDirs(state, &r)

	return r
}

func checkArrDownloadClient(state *SetupState, r *StepResult, service, expectedCategory string) {
	ver := config.ServiceAPIVer(service)

	var clients []downloadClient
	if err := api.GetJSON(service, fmt.Sprintf("api/%s/downloadclient", ver), nil, &clients); err != nil {
		r.errf("%s: cannot query download clients: %v", service, err)
		return
	}

	// Find qBittorrent client
	var qbitClient *downloadClient
	for i := range clients {
		if clients[i].Implementation == "QBittorrent" {
			qbitClient = &clients[i]
			break
		}
	}

	if qbitClient == nil {
		fmt.Printf("  %s [%s] No qBittorrent download client configured\n", ui.Err("✗"), service)

		var confirm bool
		form := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Create qBittorrent download client in %s?", service)).
				Value(&confirm),
		))
		if err := form.Run(); err != nil || !confirm {
			r.errf("%s: no qBittorrent download client", service)
			return
		}

		if createQbitClient(state, r, service, expectedCategory) {
			r.fix()
		}
		return
	}

	// Validate existing client settings
	host := fmt.Sprintf("%v", qbitClient.getField("host"))
	port := qbitClient.getField("port")
	var category string
	categoryField := "movieCategory"
	if service == "sonarr" {
		categoryField = "tvCategory"
	}
	if v := qbitClient.getField(categoryField); v != nil {
		category = fmt.Sprintf("%v", v)
	}

	issues := []string{}
	if host != "localhost" && host != "127.0.0.1" {
		issues = append(issues, fmt.Sprintf("host=%s (should be localhost)", host))
	}
	if fmt.Sprintf("%v", port) != "8080" {
		issues = append(issues, fmt.Sprintf("port=%v (expected 8080)", port))
	}
	if category != expectedCategory {
		issues = append(issues, fmt.Sprintf("category=%s (should be %s)", category, expectedCategory))
	}

	if len(issues) == 0 {
		fmt.Printf("  %s [%s] qBittorrent client OK (host=%s, category=%s)\n",
			ui.Ok("✓"), service, host, category)
		r.pass()
		return
	}

	fmt.Printf("  %s [%s] qBittorrent client issues: %s\n",
		ui.Warn("!"), service, strings.Join(issues, ", "))

	var confirm bool
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(fmt.Sprintf("Fix %s download client settings?", service)).
			Value(&confirm),
	))
	if err := form.Run(); err != nil || !confirm {
		r.errf("%s: download client misconfigured: %s", service, strings.Join(issues, ", "))
		return
	}

	// Fix the fields
	qbitClient.setField("host", "localhost")
	qbitClient.setField("port", 8080)
	qbitClient.setField(categoryField, expectedCategory)

	endpoint := fmt.Sprintf("api/%s/downloadclient/%d", ver, qbitClient.ID)
	if _, err := api.Put(service, endpoint, qbitClient, nil); err != nil {
		r.errf("%s: failed to update download client: %v", service, err)
		return
	}

	fmt.Printf("  %s [%s] Download client fixed\n", ui.Ok("✓"), service)
	r.fix()
}

func createQbitClient(state *SetupState, r *StepResult, service, category string) bool {
	ver := config.ServiceAPIVer(service)

	// Get schema for QBittorrent implementation
	var schemas []downloadClient
	if err := api.GetJSON(service, fmt.Sprintf("api/%s/downloadclient/schema", ver), nil, &schemas); err != nil {
		r.errf("%s: cannot get download client schema: %v", service, err)
		return false
	}

	var schema *downloadClient
	for i := range schemas {
		if schemas[i].Implementation == "QBittorrent" {
			schema = &schemas[i]
			break
		}
	}
	if schema == nil {
		r.errf("%s: QBittorrent schema not found", service)
		return false
	}

	// Configure the client
	schema.Name = "qBittorrent"
	schema.Enable = true
	schema.Priority = 1
	schema.RemoveCompletedDownloads = true
	schema.RemoveFailedDownloads = true
	schema.setField("host", "localhost")
	schema.setField("port", 8080)
	schema.setField("username", "admin")

	categoryField := "movieCategory"
	if service == "sonarr" {
		categoryField = "tvCategory"
	}
	schema.setField(categoryField, category)

	// Prompt for password
	var password string
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("qBittorrent Web UI password (for " + service + ")").
			Value(&password),
	))
	if err := form.Run(); err != nil {
		r.errf("%s: cancelled password entry", service)
		return false
	}
	schema.setField("password", password)

	endpoint := fmt.Sprintf("api/%s/downloadclient", ver)
	if _, err := api.Post(service, endpoint, schema, nil); err != nil {
		r.errf("%s: failed to create download client: %v", service, err)
		return false
	}

	fmt.Printf("  %s [%s] qBittorrent download client created\n", ui.Ok("✓"), service)
	return true
}

func checkQbitCategories(state *SetupState, r *StepResult) {
	fmt.Printf("\n  %s\n", ui.Bold("  qBittorrent Categories"))

	mediaWin := state.MediaWin
	if mediaWin == "" {
		mediaWin = config.MediaPathWin()
	}
	// qBittorrent uses forward slashes even on Windows
	mediaFwd := strings.ReplaceAll(mediaWin, `\`, "/")

	expectedCategories := map[string]string{
		"movies":    mediaFwd + "/Downloads/movies",
		"tv-sonarr": mediaFwd + "/Downloads/tv",
	}

	// Try to fix via qBittorrent API first (takes effect immediately)
	qbitURL := config.ServiceURL("qbittorrent")

	for catName, expectedPath := range expectedCategories {
		// Check current category via API
		c := &http.Client{Timeout: 5 * time.Second}
		resp, err := c.Get(qbitURL + "/api/v2/torrents/categories")
		if err != nil {
			// Fall back to file-based fix
			fixCategoriesFile(state, r, expectedCategories)
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		var categories map[string]struct {
			SavePath string `json:"savePath"`
		}
		if err := json.Unmarshal(body, &categories); err != nil {
			fixCategoriesFile(state, r, expectedCategories)
			return
		}

		cat, exists := categories[catName]
		if exists && (cat.SavePath == expectedPath || normalizePath(cat.SavePath) == normalizePath(expectedPath)) {
			fmt.Printf("  %s %s → %s\n", ui.Ok("✓"), catName, expectedPath)
			r.pass()
			continue
		}

		current := "(not set)"
		if exists {
			current = cat.SavePath
		}
		fmt.Printf("  %s %s → %s (expected %s)\n", ui.Warn("!"), catName, current, expectedPath)

		var confirm bool
		form := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Fix %s category path to %s?", catName, expectedPath)).
				Value(&confirm),
		))
		if err := form.Run(); err != nil || !confirm {
			r.errf("qBittorrent category %s has wrong save path", catName)
			continue
		}

		// Create or edit category via API
		params := url.Values{}
		params.Set("category", catName)
		params.Set("savePath", expectedPath)

		var apiEndpoint string
		if exists {
			apiEndpoint = "/api/v2/torrents/editCategory"
		} else {
			apiEndpoint = "/api/v2/torrents/createCategory"
		}

		req, _ := http.NewRequest("POST", qbitURL+apiEndpoint, strings.NewReader(params.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp2, err := c.Do(req)
		if err != nil {
			r.errf("failed to fix qBittorrent category %s via API: %v", catName, err)
			continue
		}
		resp2.Body.Close()

		fmt.Printf("  %s %s → %s (fixed)\n", ui.Ok("✓"), catName, expectedPath)
		r.fix()
	}
}

func fixCategoriesFile(state *SetupState, r *StepResult, expected map[string]string) {
	// Find categories.json
	home := os.Getenv("HOME")
	_ = home
	catPaths := []string{
		"/mnt/c/Users/Max/AppData/Roaming/qBittorrent/categories.json",
	}

	// Try to find dynamically
	matches, _ := filepath.Glob("/mnt/c/Users/*/AppData/Roaming/qBittorrent/categories.json")
	if len(matches) > 0 {
		catPaths = matches
	}

	var catFile string
	for _, p := range catPaths {
		if _, err := os.Stat(p); err == nil {
			catFile = p
			break
		}
	}

	if catFile == "" {
		r.err("cannot find qBittorrent categories.json")
		return
	}

	data, err := os.ReadFile(catFile)
	if err != nil {
		r.errf("cannot read %s: %v", catFile, err)
		return
	}

	var categories map[string]qBitCategory
	if err := json.Unmarshal(data, &categories); err != nil {
		categories = make(map[string]qBitCategory)
	}

	changed := false
	for name, path := range expected {
		cat, exists := categories[name]
		if !exists || normalizePath(cat.SavePath) != normalizePath(path) {
			categories[name] = qBitCategory{SavePath: path}
			changed = true
		}
	}

	if !changed {
		r.pass()
		return
	}

	var confirm bool
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Fix qBittorrent categories.json?").
			Value(&confirm),
	))
	if err := form.Run(); err != nil || !confirm {
		r.err("qBittorrent categories not fixed")
		return
	}

	out, err := json.MarshalIndent(categories, "", "    ")
	if err != nil {
		r.errf("cannot marshal categories: %v", err)
		return
	}
	if err := os.WriteFile(catFile, append(out, '\n'), 0644); err != nil {
		r.errf("cannot write %s: %v", catFile, err)
		return
	}

	fmt.Printf("  %s Updated %s\n", ui.Ok("✓"), ui.Dim(catFile))
	r.fix()
}

func checkQbitDefaultPath(state *SetupState, r *StepResult) {
	fmt.Printf("\n  %s\n", ui.Bold("  qBittorrent Default Save Path"))

	qbitURL := config.ServiceURL("qbittorrent")
	c := &http.Client{Timeout: 5 * time.Second}
	resp, err := c.Get(qbitURL + "/api/v2/app/preferences")
	if err != nil {
		r.errf("cannot query qBittorrent preferences: %v", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var prefs struct {
		SavePath string `json:"save_path"`
	}
	if err := json.Unmarshal(body, &prefs); err != nil {
		r.errf("cannot parse qBittorrent preferences: %v", err)
		return
	}

	mediaWin := state.MediaWin
	if mediaWin == "" {
		mediaWin = config.MediaPathWin()
	}
	expectedPath := strings.ReplaceAll(mediaWin, `\`, "/") + "/Downloads"

	if normalizePath(prefs.SavePath) == normalizePath(expectedPath) {
		fmt.Printf("  %s Default save path: %s\n", ui.Ok("✓"), prefs.SavePath)
		r.pass()
		return
	}

	fmt.Printf("  %s Default save path: %s (expected %s)\n", ui.Warn("!"), prefs.SavePath, expectedPath)

	var confirm bool
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(fmt.Sprintf("Fix default save path to %s?", expectedPath)).
			Value(&confirm),
	))
	if err := form.Run(); err != nil || !confirm {
		r.errf("qBittorrent default save path is %s (should be %s)", prefs.SavePath, expectedPath)
		return
	}

	payload := fmt.Sprintf(`json={"save_path":"%s"}`, expectedPath)
	req, _ := http.NewRequest("POST", qbitURL+"/api/v2/app/setPreferences",
		strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp2, err := c.Do(req)
	if err != nil {
		r.errf("failed to fix qBittorrent save path: %v", err)
		return
	}
	resp2.Body.Close()

	fmt.Printf("  %s Default save path fixed to %s\n", ui.Ok("✓"), expectedPath)
	r.fix()
}

func checkDownloadDirs(state *SetupState, r *StepResult) {
	mediaWSL := state.MediaWSL
	if mediaWSL == "" {
		mediaWSL = config.MediaPathWSL()
	}

	dirs := []string{
		filepath.Join(mediaWSL, "Downloads"),
		filepath.Join(mediaWSL, "Downloads", "movies"),
		filepath.Join(mediaWSL, "Downloads", "tv"),
	}

	for _, dir := range dirs {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			r.pass()
			continue
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			// Try via PowerShell for Windows filesystem
			winPath := wslToWin(dir)
			cmd := fmt.Sprintf(`New-Item -Path '%s' -ItemType Directory -Force`, winPath)
			out, err2 := execPowershell(cmd)
			if err2 != nil {
				r.errf("cannot create %s: %v / %s", dir, err, out)
				continue
			}
		}
		fmt.Printf("  %s Created %s\n", ui.Ok("✓"), dir)
		r.fix()
	}
}

func normalizePath(p string) string {
	p = strings.ReplaceAll(p, `\`, "/")
	p = strings.TrimRight(p, "/")
	return strings.ToLower(p)
}

func wslToWin(path string) string {
	if strings.HasPrefix(path, "/mnt/") && len(path) > 6 {
		drive := strings.ToUpper(string(path[5]))
		rest := strings.ReplaceAll(path[6:], "/", `\`)
		return drive + ":" + rest
	}
	return path
}
