package config

import (
	"testing"
)

func TestDefaultDownloadClients_AllServices(t *testing.T) {
	expected := []string{"radarr", "sonarr", "lidarr", "readarr", "whisparr"}
	if len(DefaultDownloadClients) != len(expected) {
		t.Fatalf("expected %d download client defs, got %d", len(expected), len(DefaultDownloadClients))
	}
	for i, svc := range expected {
		if DefaultDownloadClients[i].Service != svc {
			t.Errorf("index %d: expected service %s, got %s", i, svc, DefaultDownloadClients[i].Service)
		}
		if DefaultDownloadClients[i].Category == "" {
			t.Errorf("%s: category must not be empty", svc)
		}
		if DefaultDownloadClients[i].CategoryField == "" {
			t.Errorf("%s: categoryField must not be empty", svc)
		}
		if DefaultDownloadClients[i].TorrentDir == "" {
			t.Errorf("%s: torrentDir must not be empty", svc)
		}
	}
}

func TestDownloadClientFor(t *testing.T) {
	dc, ok := DownloadClientFor("radarr")
	if !ok {
		t.Fatal("expected to find radarr download client def")
	}
	if dc.Category != "movies" {
		t.Errorf("radarr: expected category=movies, got %s", dc.Category)
	}
	if dc.CategoryField != "movieCategory" {
		t.Errorf("radarr: expected categoryField=movieCategory, got %s", dc.CategoryField)
	}

	dc, ok = DownloadClientFor("sonarr")
	if !ok {
		t.Fatal("expected to find sonarr download client def")
	}
	if dc.Category != "tv-sonarr" {
		t.Errorf("sonarr: expected category=tv-sonarr, got %s", dc.Category)
	}

	_, ok = DownloadClientFor("nonexistent")
	if ok {
		t.Error("expected nonexistent service to return false")
	}
}

func TestDefaultRootFolders_AllServices(t *testing.T) {
	expected := []string{"radarr", "sonarr", "lidarr", "readarr", "whisparr"}
	if len(DefaultRootFolders) != len(expected) {
		t.Fatalf("expected %d root folder defs, got %d", len(expected), len(DefaultRootFolders))
	}
	for i, svc := range expected {
		if DefaultRootFolders[i].Service != svc {
			t.Errorf("index %d: expected service %s, got %s", i, svc, DefaultRootFolders[i].Service)
		}
		if DefaultRootFolders[i].Subdir == "" {
			t.Errorf("%s: subdir must not be empty", svc)
		}
	}
}

func TestRootFolderFor(t *testing.T) {
	rf, ok := RootFolderFor("radarr")
	if !ok {
		t.Fatal("expected to find radarr root folder def")
	}
	if rf.Subdir != "media/movies" {
		t.Errorf("radarr: expected subdir=media/movies, got %s", rf.Subdir)
	}

	_, ok = RootFolderFor("nonexistent")
	if ok {
		t.Error("expected nonexistent service to return false")
	}
}

func TestRecommendedIndexers_NotEmpty(t *testing.T) {
	if len(RecommendedIndexers) == 0 {
		t.Fatal("RecommendedIndexers should not be empty")
	}

	// Check all have required fields
	for _, idx := range RecommendedIndexers {
		if idx.Name == "" {
			t.Error("indexer has empty name")
		}
		if idx.Category == "" {
			t.Errorf("indexer %s: category must not be empty", idx.Name)
		}
		if idx.Implementation == "" {
			t.Errorf("indexer %s: implementation must not be empty", idx.Name)
		}
		if idx.ConfigContract == "" {
			t.Errorf("indexer %s: configContract must not be empty", idx.Name)
		}
	}
}

func TestRecommendedIndexers_Categories(t *testing.T) {
	validCats := map[string]bool{"general": true, "movies": true, "tv": true, "anime": true}
	for _, idx := range RecommendedIndexers {
		if !validCats[idx.Category] {
			t.Errorf("indexer %s: invalid category %q", idx.Name, idx.Category)
		}
	}
}

func TestLookupRecommendedIndexer(t *testing.T) {
	// Exact match
	def, ok := LookupRecommendedIndexer("1337x")
	if !ok {
		t.Fatal("expected to find 1337x")
	}
	if def.Category != "general" {
		t.Errorf("1337x: expected category=general, got %s", def.Category)
	}

	// Case-insensitive
	def, ok = LookupRecommendedIndexer("yts")
	if !ok {
		t.Fatal("expected case-insensitive lookup to find YTS")
	}
	if def.Name != "YTS" {
		t.Errorf("expected canonical name YTS, got %s", def.Name)
	}

	// Not found
	_, ok = LookupRecommendedIndexer("nonexistent")
	if ok {
		t.Error("expected nonexistent indexer to return false")
	}
}

func TestDefToIndexerConfig(t *testing.T) {
	def := IndexerDef{
		Name:       "Test",
		NeedsFlare: true,
		BaseURL:    "https://example.com",
		ExtraFields: map[string]interface{}{
			"key": "value",
		},
	}

	ic := DefToIndexerConfig(def)

	if !ic.Flare {
		t.Error("expected Flare=true")
	}
	if ic.BaseURL != "https://example.com" {
		t.Errorf("expected BaseURL=https://example.com, got %s", ic.BaseURL)
	}
	if ic.ExtraFields["key"] != "value" {
		t.Errorf("expected ExtraFields[key]=value, got %v", ic.ExtraFields["key"])
	}
}

func TestDownloadClientsAndRootFolders_SameServices(t *testing.T) {
	// Verify both tables define the same services in the same order
	if len(DefaultDownloadClients) != len(DefaultRootFolders) {
		t.Fatalf("download clients (%d) and root folders (%d) have different counts",
			len(DefaultDownloadClients), len(DefaultRootFolders))
	}
	for i := range DefaultDownloadClients {
		if DefaultDownloadClients[i].Service != DefaultRootFolders[i].Service {
			t.Errorf("index %d: download client service=%s, root folder service=%s",
				i, DefaultDownloadClients[i].Service, DefaultRootFolders[i].Service)
		}
	}
}
