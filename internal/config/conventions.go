package config

import "strings"

// DownloadClientDef maps an *Arr service to its qBittorrent category settings.
type DownloadClientDef struct {
	Service       string
	Category      string
	CategoryField string
	TorrentDir    string
}

// DefaultDownloadClients defines the qBittorrent category for each *Arr service.
var DefaultDownloadClients = []DownloadClientDef{
	{"radarr", "movies", "movieCategory", "torrents/movies"},
	{"sonarr", "tv-sonarr", "tvCategory", "torrents/tv"},
	{"lidarr", "music", "musicCategory", "torrents/music"},
	{"readarr", "books", "bookCategory", "torrents/books"},
	{"whisparr", "xxx", "movieCategory", "torrents/xxx"},
}

// DownloadClientFor returns the download client definition for a given service.
func DownloadClientFor(service string) (DownloadClientDef, bool) {
	for _, dc := range DefaultDownloadClients {
		if dc.Service == service {
			return dc, true
		}
	}
	return DownloadClientDef{}, false
}

// RootFolderDef maps an *Arr service to its expected media subdirectory.
type RootFolderDef struct {
	Service string
	Subdir  string
}

// DefaultRootFolders defines the root folder path for each *Arr service.
var DefaultRootFolders = []RootFolderDef{
	{"radarr", "media/movies"},
	{"sonarr", "media/tv"},
	{"lidarr", "media/music"},
	{"readarr", "media/books"},
	{"whisparr", "media/xxx"},
}

// RootFolderFor returns the root folder definition for a given service.
func RootFolderFor(service string) (RootFolderDef, bool) {
	for _, rf := range DefaultRootFolders {
		if rf.Service == service {
			return rf, true
		}
	}
	return RootFolderDef{}, false
}

// IndexerDef describes a recommended indexer with its Prowlarr configuration.
type IndexerDef struct {
	Name           string
	Category       string // "general", "movies", "tv", "anime"
	Implementation string
	ConfigContract string
	DefinitionFile string
	BaseURL        string
	NeedsFlare     bool
	ExtraFields    map[string]interface{}
}

// RecommendedIndexers is the canonical list of recommended indexers.
var RecommendedIndexers = []IndexerDef{
	// General
	{Name: "1337x", Category: "general", Implementation: "Cardigann", ConfigContract: "CardigannSettings",
		DefinitionFile: "1337x", NeedsFlare: true},
	{Name: "The Pirate Bay", Category: "general", Implementation: "Cardigann", ConfigContract: "CardigannSettings",
		DefinitionFile: "thepiratebay", BaseURL: "https://thepiratebay.org/",
		ExtraFields: map[string]interface{}{"apiurl": "apibay.org"}},
	{Name: "TorrentGalaxy", Category: "general", Implementation: "Cardigann", ConfigContract: "CardigannSettings",
		DefinitionFile: "torrentgalaxyclone", BaseURL: "https://torrentgalaxy.info/"},
	{Name: "Knaben", Category: "general", Implementation: "Knaben", ConfigContract: "NoAuthTorrentBaseSettings",
		BaseURL: "https://knaben.org/"},

	// Movies
	{Name: "YTS", Category: "movies", Implementation: "Cardigann", ConfigContract: "CardigannSettings",
		DefinitionFile: "yts"},

	// TV
	{Name: "EZTV", Category: "tv", Implementation: "Cardigann", ConfigContract: "CardigannSettings",
		DefinitionFile: "eztv"},

	// Anime
	{Name: "Nyaa.si", Category: "anime", Implementation: "Cardigann", ConfigContract: "CardigannSettings",
		DefinitionFile: "nyaasi", BaseURL: "https://nyaa.si/", NeedsFlare: true},
	{Name: "SubsPlease", Category: "anime", Implementation: "SubsPlease", ConfigContract: "SubsPleaseSettings"},
	{Name: "Anidex", Category: "anime", Implementation: "Anidex", ConfigContract: "AnidexSettings",
		BaseURL: "https://anidex.info/", NeedsFlare: true,
		ExtraFields: map[string]interface{}{"authorisedOnly": false}},
	{Name: "Tokyo Toshokan", Category: "anime", Implementation: "Cardigann", ConfigContract: "CardigannSettings",
		DefinitionFile: "tokyotosho", BaseURL: "https://www.tokyotosho.info/"},
}

// LookupRecommendedIndexer finds an IndexerDef by name (case-insensitive).
func LookupRecommendedIndexer(name string) (IndexerDef, bool) {
	for _, def := range RecommendedIndexers {
		if strings.EqualFold(def.Name, name) {
			return def, true
		}
	}
	return IndexerDef{}, false
}

// DefToIndexerConfig converts an IndexerDef to an IndexerConfig.
func DefToIndexerConfig(def IndexerDef) IndexerConfig {
	return IndexerConfig{
		Flare:       def.NeedsFlare,
		BaseURL:     def.BaseURL,
		ExtraFields: def.ExtraFields,
	}
}
