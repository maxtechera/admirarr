<p align="center">
  <br>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/logo-dark.svg">
    <source media="(prefers-color-scheme: light)" srcset="assets/logo.svg">
    <img alt="Admirarr" src="assets/logo.svg" width="180">
  </picture>
  <br><br>
  <a href="#install"><img src="https://img.shields.io/badge/install-one_liner-D4A843?style=for-the-badge" alt="Install"></a>
  &nbsp;
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?style=for-the-badge" alt="License"></a>
  &nbsp;
  <img src="https://img.shields.io/badge/python-3.7+-3776AB?style=for-the-badge&logo=python&logoColor=white" alt="Python">
  &nbsp;
  <img src="https://img.shields.io/badge/deps-zero-2ECC71?style=for-the-badge" alt="Dependencies">
  <br><br>
  <strong>A fast, zero-dependency CLI for managing your entire Plex + *Arr media server stack.<br>One command to rule the seven seas.</strong>
</p>

<br>

<p align="center">
  <img src="https://asciinema.org/a/YZ4Xqg6qsNuvMZpf.svg" alt="admirarr status demo" width="700">
</p>

<br>

## Highlights

<table>
<tr>
<td width="50%" valign="top">

### Fleet Dashboard
One command to see every service, library stat, queue, and disk — at a glance.

```
$ admirarr status
```

</td>
<td width="50%" valign="top">

### Ship Doctor + AI Fix Wizard
Full diagnostics with auto-fix. Works with Claude Code, Aider, OpenCode, or Goose.

```
$ admirarr doctor --fix
```

</td>
</tr>
<tr>
<td width="50%" valign="top">

### Search & Add
Find and add movies or shows directly from the terminal. No browser needed.

```
$ admirarr add-movie "interstellar"
```

</td>
<td width="50%" valign="top">

### Download Watch
Monitor qBittorrent torrents and Radarr/Sonarr import queues in real time.

```
$ admirarr downloads
```

</td>
</tr>
</table>

<br>

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/maxtechera/admirarr/main/install.sh | bash
```

<details>
<summary><strong>More install options</strong></summary>

#### Custom directory

```bash
ADMIRARR_INSTALL_DIR=~/.local/bin curl -fsSL https://raw.githubusercontent.com/maxtechera/admirarr/main/install.sh | bash
```

#### Manual

```bash
curl -sL https://raw.githubusercontent.com/maxtechera/admirarr/main/admirarr -o admirarr
chmod +x admirarr
sudo mv admirarr /usr/local/bin/
```

#### Requirements

- Python 3.7+
- Your *Arr stack running on the same machine or reachable over the network
- That's it. No pip, no venv, no node_modules.

</details>

<br>

## Commands

| Command | What it does |
|---------|-------------|
| `admirarr status` | Dashboard: services, library, queues, disk |
| `admirarr doctor` | Run full diagnostics on your stack |
| `admirarr doctor --fix` | Auto-fix issues with an AI agent (Claude Code, Aider, etc.) |
| `admirarr health` | Health warnings from Radarr/Sonarr/Prowlarr |

<details>
<summary><strong>Library</strong></summary>

| Command | What it does |
|---------|-------------|
| `admirarr movies` | List all movies in Radarr |
| `admirarr shows` | List all TV shows in Sonarr |
| `admirarr missing` | Monitored content without files |
| `admirarr recent` | Recently added to Plex |
| `admirarr history` | Tautulli watch history |

</details>

<details>
<summary><strong>Search & Add</strong></summary>

| Command | What it does |
|---------|-------------|
| `admirarr search <query>` | Search Prowlarr indexers |
| `admirarr find <query>` | Search Radarr releases for a movie |
| `admirarr add-movie <query>` | Search and add a movie to Radarr |
| `admirarr add-show <query>` | Search and add a TV show to Sonarr |

</details>

<details>
<summary><strong>Downloads</strong></summary>

| Command | What it does |
|---------|-------------|
| `admirarr downloads` | Active qBittorrent torrents |
| `admirarr queue` | Radarr + Sonarr import queues |

</details>

<details>
<summary><strong>Infrastructure</strong></summary>

| Command | What it does |
|---------|-------------|
| `admirarr indexers` | Prowlarr indexer status |
| `admirarr scan` | Trigger Plex library scan |
| `admirarr restart <svc>` | Restart a service |
| `admirarr docker` | Docker container status |
| `admirarr disk` | Disk space breakdown |
| `admirarr logs <svc>` | Recent logs (sonarr\|radarr) |

</details>

<br>

## Supported Services

<table>
<tr>
<td align="center" width="100"><strong>Plex</strong><br><sub>Media Server</sub></td>
<td align="center" width="100"><strong>Radarr</strong><br><sub>Movies</sub></td>
<td align="center" width="100"><strong>Sonarr</strong><br><sub>TV Shows</sub></td>
<td align="center" width="100"><strong>Prowlarr</strong><br><sub>Indexers</sub></td>
<td align="center" width="100"><strong>qBittorrent</strong><br><sub>Downloads</sub></td>
</tr>
<tr>
<td align="center" width="100"><strong>Tautulli</strong><br><sub>Analytics</sub></td>
<td align="center" width="100"><strong>Seerr</strong><br><sub>Requests</sub></td>
<td align="center" width="100"><strong>Bazarr</strong><br><sub>Subtitles</sub></td>
<td align="center" width="100"><strong>Organizr</strong><br><sub>Dashboard</sub></td>
<td align="center" width="100"><strong>FlareSolverr</strong><br><sub>CF Bypass</sub></td>
</tr>
</table>

Works with both **Windows services** and **Docker containers**. Runs from Linux, macOS, or WSL.

<br>

## Configuration

Admirarr reads API keys directly from your \*Arr config files — **zero manual setup** if running on the same machine or WSL.

```python
WIN_HOST = "192.168.50.42"  # Edit this to match your server IP
```

<details>
<summary><strong>API key sources</strong></summary>

| Service | Config Path |
|---------|------------|
| Sonarr | `/mnt/c/ProgramData/Sonarr/config.xml` |
| Radarr | `/mnt/c/ProgramData/Radarr/config.xml` |
| Prowlarr | `/mnt/c/ProgramData/Prowlarr/config.xml` |
| Plex | `Preferences.xml` (auto-detected) |
| Tautulli | `/mnt/c/ProgramData/Tautulli/config.ini` |
| Seerr | Read from Docker container |

</details>

<br>

## How It Works

A single Python file (~900 lines), zero external dependencies:

```
urllib.request     HTTP/API calls
xml.etree          Plex XML parsing
subprocess         Docker & Windows service management
ANSI escapes       Colored terminal output (NO_COLOR supported)
```

No pip. No venv. No node_modules. Just Python 3.7+ and your \*Arr stack.

<br>

## Contributing

```bash
git clone https://github.com/maxtechera/admirarr.git
cd admirarr
python3 test_admirarr.py   # Run tests
```

<br>

## License

[MIT](LICENSE)

---

<p align="center">
  <sub>Built with <img src="assets/icon.svg" width="16" height="16" style="vertical-align: middle;"> by <a href="https://github.com/maxtechera">maxtechera</a></sub>
</p>
