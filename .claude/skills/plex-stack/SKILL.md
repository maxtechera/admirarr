---
name: plex-stack
description: Manage the Plex + *Arr media server stack — check status, request content, monitor downloads, manage libraries, and troubleshoot services
user-invocable: true
argument-hint: [status|doctor|search|downloads|fix]
allowed-tools: Bash, Read, Grep, Glob
---

# Plex + *Arr Stack Management

You are managing a Windows-based media server stack accessed from WSL.

## Network

- Windows host: `192.168.50.42`
- Docker: `localhost`

| Service | Port | Type |
|---------|------|------|
| Plex | 32400 | Windows |
| Radarr | 7878 | Windows |
| Sonarr | 8989 | Windows |
| Prowlarr | 9696 | Windows |
| qBittorrent | 8080 | Windows |
| Tautulli | 8181 | Windows |
| Seerr | 5055 | Docker |
| Bazarr | 6767 | Docker |
| Organizr | 9983 | Docker |
| FlareSolverr | 8191 | Docker |

## API Keys

```bash
SONARR_KEY=$(grep -oPm1 '(?<=<ApiKey>)[^<]+' /mnt/c/ProgramData/Sonarr/config.xml 2>/dev/null)
RADARR_KEY=$(grep -oPm1 '(?<=<ApiKey>)[^<]+' /mnt/c/ProgramData/Radarr/config.xml 2>/dev/null)
PROWLARR_KEY=$(grep -oPm1 '(?<=<ApiKey>)[^<]+' /mnt/c/ProgramData/Prowlarr/config.xml 2>/dev/null)
PLEX_TOKEN=$(grep -oP 'PlexOnlineToken="[^"]*"' /mnt/c/Users/*/AppData/Local/Plex\ Media\ Server/Preferences.xml 2>/dev/null | head -1 | grep -oP '"\K[^"]+')
```

## Media Paths

- WSL: `/mnt/d/Media`
- Windows: `D:\Media`
- Structure: `Downloads/{tv,movies,incomplete}`, `TV Shows/`, `Movies/`

## Quick Commands

Based on `$ARGUMENTS`:

- **status**: Check all services, library counts, queues, disk
- **doctor**: Run full diagnostics (9 categories)
- **search <query>**: Search Radarr/Sonarr for content
- **downloads**: Show active qBittorrent torrents
- **fix**: Run doctor and fix issues

## Rules

- Always get API keys before making calls
- Confirm before destructive actions
- Use `jq` for JSON, `python3` for XML
- Present data in clean tables, not raw JSON
