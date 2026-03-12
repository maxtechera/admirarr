# Admirarr — Agent Instructions

## Overview

Admirarr is a Go CLI (Cobra + Charm.sh) for managing a Plex + *Arr media server stack from the terminal.

## Architecture

- **Go 1.24+** with spf13/cobra, spf13/viper, charmbracelet/lipgloss, charmbracelet/huh
- **Structure**: `main.go` → `cmd/` (20 command files) → `internal/` (api, config, keys, ui, doctor)
- **Config**: Viper loads `~/.config/admirarr/config.yaml`
- **API pattern**: `api.GetJSON(service, endpoint, params, &target)`

## Network

- Windows services (Plex:32400, Radarr:7878, Sonarr:8989, Prowlarr:9696, qBittorrent:8080, Tautulli:8181) → `192.168.50.42`
- Docker containers (Seerr:5055, Bazarr:6767, Organizr:9983, FlareSolverr:8191) → `localhost`
- Media: `/mnt/d/Media` (WSL) / `D:\Media` (Windows)

## Commands

```
admirarr status|doctor|doctor --fix|health|movies|shows|missing|recent|history
admirarr search|find|add-movie|add-show <query>
admirarr downloads|queue|indexers|scan|restart|docker|disk|logs
admirarr completion [bash|zsh|fish|powershell]
```

## Build & Test

```bash
go build -ldflags "-X github.com/maxtechera/admirarr/internal/ui.Version=1.0.0" -o admirarr .
go test ./...
```

## Brand

- **Name**: Admirarr (admiral + arr)
- **Tagline**: Command your fleet.
- **Theme**: Navy + Gold + Cyan, pirate/nautical, octopus mascot
