# Admirarr — Project Instructions

## Overview

Admirarr is a Go CLI for managing a Plex + *Arr media server stack from the terminal. It targets the self-hosted/homelab community. Built with Cobra + Charm.sh for a polished terminal experience.

## Architecture

- **Go 1.24+** with Cobra command framework
- **4 runtime deps**: spf13/cobra, spf13/viper, charmbracelet/lipgloss, charmbracelet/huh
- **Config**: Viper loads `~/.config/admirarr/config.yaml`, falls back to hardcoded defaults
- **API pattern**: `api.GetJSON(service, endpoint, params, &target)` handles HTTP + auth injection
- **Key auto-discovery**: Reads API keys from *Arr config.xml files on Windows filesystem via WSL mounts
- **Version injection**: `ldflags -X github.com/maxtechera/admirarr/internal/ui.Version=X.Y.Z`

## Network Architecture

- Windows services (Plex, Radarr, Sonarr, Prowlarr, qBittorrent, Tautulli) at `192.168.50.42`
- Docker containers (Seerr, Bazarr, Organizr, FlareSolverr) at `localhost`
- Media path: `/mnt/d/Media` (WSL) / `D:\Media` (Windows)

## Project Structure

```
main.go                    # cmd.Execute()
cmd/                       # One file per command (20 commands)
internal/api/client.go     # HTTP client with auth injection
internal/config/config.go  # Viper config management
internal/keys/keys.go      # API key auto-discovery
internal/ui/               # Lip Gloss styles, formatting, banner
internal/doctor/           # Diagnostic checks + AI fix wizard
```

## Build and Test

```bash
go build -ldflags "-X github.com/maxtechera/admirarr/internal/ui.Version=1.0.0" -o admirarr .
go test ./...
./admirarr --help
```

## Distribution

GoReleaser builds for linux/darwin/windows (amd64/arm64) and publishes to:
- GitHub Releases (binaries, deb, rpm)
- Homebrew (maxtechera/tap)
- Scoop (maxtechera/scoop-bucket)
- Docker (ghcr.io/maxtechera/admirarr)
- Snap Store
- AUR (admirarr-bin)

Tag `v*` on main triggers the release workflow.

## Brand Identity

- **Name**: Admirarr (admiral + arr, pirate/nautical theme)
- **Tagline**: "Command your fleet."
- **Mascot**: Octopus wearing an admiral's tricorn hat
- **Colors**: Navy (#0A1628, #0F2040), Gold (#D4A843, #F0D080), Cyan (#00BCD4)
- **Anchor emoji**: ⚓ used as the brand icon in terminal output

## Supported Services

Plex, Radarr, Sonarr, Prowlarr, qBittorrent, Tautulli, Seerr, Bazarr, Organizr, FlareSolverr
