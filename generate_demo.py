#!/usr/bin/env python3
"""
Generate a realistic admirarr demo cast file.
Shows: setup flow → status → doctor → search
Duration: ~45 seconds
"""

import json
import sys

ESC = "\u001b"
GOLD = ESC + "[33m"
BOLD = ESC + "[1m"
DIM = ESC + "[2m"
RESET = ESC + "[0m"
GREEN = ESC + "[92m"
RED = ESC + "[91m"
YELLOW = ESC + "[93m"
CYAN = ESC + "[96m"
BLUE = ESC + "[34m"
SEP = "  " + DIM + "━" * 50 + RESET

def sep():
    return SEP + "\r\n"

def bold(s):
    return BOLD + s + RESET

def gold(s):
    return GOLD + s + RESET

def dim(s):
    return DIM + s + RESET

def ok(s):
    return GREEN + s + RESET

def err(s):
    return RED + s + RESET

def warn(s):
    return YELLOW + s + RESET

def banner():
    return (
        "\r\n"
        "  " + GOLD + "⚓ " + BOLD + "ADMIRARR" + RESET + " " + DIM + "v1.0.0" + RESET + "    " + DIM + "Command your fleet." + RESET + "\r\n"
        "  " + DIM + "━" * 50 + RESET + "\r\n"
        "\r\n"
    )


# Build events: [time, "o", text]
events = []
t = 0.0

def add(text, delay=0.0):
    global t
    t += delay
    events.append([round(t, 6), "o", text])

def pause(d):
    """Add a pause (no output)"""
    global t
    t += d

def type_cmd(cmd, pre_delay=1.5):
    """Simulate typing a command with realistic delays"""
    global t
    t += pre_delay
    # Show prompt
    events.append([round(t, 6), "o", "\r\n" + DIM + "$ " + RESET])
    t += 0.15
    # Type each char with realistic variance
    for i, ch in enumerate(cmd):
        # Vary timing per char
        if ch == ' ':
            t += 0.09
        elif i == 0:
            t += 0.2
        else:
            t += 0.06 + (0.02 if i % 3 == 0 else 0)
        events.append([round(t, 6), "o", ch])
    t += 0.5
    events.append([round(t, 6), "o", "\r\n"])
    t += 0.15


# ─── SECTION 1: admirarr setup ───────────────────────────────────────────────
type_cmd("admirarr setup", pre_delay=0.8)

add(banner(), 0.4)
add("  " + bold("Quick Setup — Auto-detect your *Arr stack") + "\r\n", 0.1)
add(sep(), 0.0)
add("\r\n", 0.1)
add("  Scanning network for media services...\r\n", 0.3)
add("\r\n", 0.6)
add("  " + ok("✓") + " Plex          " + dim("192.168.50.42:32400") + "  " + dim("6ms") + "\r\n", 0.3)
add("  " + ok("✓") + " Radarr        " + dim("192.168.50.42:7878") + "   " + dim("4ms") + "\r\n", 0.2)
add("  " + ok("✓") + " Sonarr        " + dim("192.168.50.42:8989") + "   " + dim("6ms") + "\r\n", 0.2)
add("  " + ok("✓") + " Prowlarr      " + dim("192.168.50.42:9696") + "   " + dim("5ms") + "\r\n", 0.2)
add("  " + ok("✓") + " qBittorrent   " + dim("192.168.50.42:8080") + "   " + dim("11ms") + "\r\n", 0.2)
add("  " + ok("✓") + " Bazarr        " + dim("192.168.50.42:6767") + "   " + dim("4ms") + "\r\n", 0.2)
add("\r\n", 0.4)

# API key prompts  
add("  Enter Radarr API key (or press Enter to skip): " + dim("a1b2c3d4e5f6...") + "\r\n", 0.1)
pause(0.5)
add("  Enter Sonarr API key (or press Enter to skip): " + dim("b2c3d4e5f6a1...") + "\r\n", 0.1)
pause(0.4)
add("\r\n", 0.2)
add("  " + ok("✓") + " Config saved → " + dim("~/.admirarr/config.yaml") + "\r\n", 0.3)
add("\r\n", 0.3)
add("  " + gold("⚓") + " All set! Run " + gold("admirarr status") + " to see your fleet.\r\n", 0.1)
add("\r\n", 0.2)


# ─── SECTION 2: admirarr status ──────────────────────────────────────────────
type_cmd("admirarr status", pre_delay=1.5)

add(banner(), 0.3)
add("  " + bold("Fleet Status") + "\r\n", 0.1)
add(sep(), 0.0)
add("\r\n", 0.1)

# Services — streamed in with delays
add("  " + ok("●") + " plex           " + dim(":32400") + "  " + dim("6ms") + "\r\n", 0.18)
add("  " + ok("●") + " radarr         " + dim(":7878") + "   " + dim("4ms") + "\r\n", 0.12)
add("  " + ok("●") + " sonarr         " + dim(":8989") + "   " + dim("6ms") + "\r\n", 0.12)
add("  " + ok("●") + " prowlarr       " + dim(":9696") + "   " + dim("5ms") + "\r\n", 0.12)
add("  " + ok("●") + " qbittorrent    " + dim(":8080") + "   " + dim("11ms") + "\r\n", 0.12)
add("  " + ok("●") + " bazarr         " + dim(":6767") + "   " + dim("4ms") + "\r\n", 0.12)
add("\r\n", 0.3)

# Library
add("  " + bold("Library") + "\r\n", 0.0)
add(sep(), 0.0)
add("\r\n", 0.05)
add("  " + GOLD + "Movies" + RESET + "     1,247 total   " + ok("1,189 on disk") + "   " + warn("58 missing") + "   " + dim("4.2 TB") + "\r\n", 0.25)
add("  " + GOLD + "TV Shows" + RESET + "   86 shows       " + ok("2,341 episodes") + "                  " + dim("1.8 TB") + "\r\n", 0.15)
add("\r\n", 0.3)

# Active Downloads
add("  " + bold("Active Downloads") + "\r\n", 0.0)
add(sep(), 0.0)
add("\r\n", 0.05)
add("  " + dim("[radarr]") + " " + ok("↓ downloading") + "     Dune.Part.Two.2024.2160p.UHD.BluRay.x265   " + ok("42.1 MB/s") + "\r\n", 0.25)
add("  " + dim("[radarr]") + " " + ok("↓ downloading") + "     Gladiator.II.2024.1080p.WEB-DL.x264        " + ok("12.3 MB/s") + "\r\n", 0.12)
add("  " + dim("[sonarr]") + " " + warn("⏳ importPending") + "  The.Bear.S03E01.1080p.AMZN.WEB-DL             " + dim("waiting") + "\r\n", 0.12)
add("\r\n", 0.3)

# Disk
add("  " + bold("Disk") + "\r\n", 0.0)
add(sep(), 0.0)
add("\r\n", 0.05)
add("  /media  [" + ok("████████████████████") + "░░░░░] " + ok("75%") + "  " + dim("2.4 TB free / 8.0 TB") + "\r\n", 0.25)
add("\r\n", 0.3)


# ─── SECTION 3: admirarr doctor ──────────────────────────────────────────────
type_cmd("admirarr doctor", pre_delay=1.5)

add(banner(), 0.3)
add("  Running diagnostics across your entire media stack...\r\n", 0.3)
add("\r\n", 0.5)

# Category 1: Connectivity
add("  " + bold("Service Connectivity") + "\r\n", 0.0)
add(sep(), 0.0)
add("  " + ok("✓") + " plex           " + dim(":32400") + "  " + dim("6ms") + "\r\n", 0.14)
add("  " + ok("✓") + " radarr         " + dim(":7878") + "   " + dim("4ms") + "\r\n", 0.1)
add("  " + ok("✓") + " sonarr         " + dim(":8989") + "   " + dim("6ms") + "\r\n", 0.1)
add("  " + ok("✓") + " prowlarr       " + dim(":9696") + "   " + dim("5ms") + "\r\n", 0.1)
add("  " + ok("✓") + " qbittorrent    " + dim(":8080") + "   " + dim("11ms") + "\r\n", 0.1)
add("  " + ok("✓") + " bazarr         " + dim(":6767") + "   " + dim("4ms") + "\r\n", 0.1)
add("\r\n", 0.3)

# Category 2: API Keys
add("  " + bold("API Keys") + "\r\n", 0.0)
add(sep(), 0.0)
add("  " + ok("✓") + " radarr    " + ok("valid") + "\r\n", 0.18)
add("  " + ok("✓") + " sonarr    " + ok("valid") + "\r\n", 0.1)
add("  " + ok("✓") + " prowlarr  " + ok("valid") + "\r\n", 0.1)
add("\r\n", 0.25)

# Category 3: Containers
add("  " + bold("Docker Containers") + "\r\n", 0.0)
add(sep(), 0.0)
add("  " + ok("✓") + " plex        " + ok("running") + "  " + dim("Up 12 days") + "\r\n", 0.14)
add("  " + ok("✓") + " radarr      " + ok("running") + "  " + dim("Up 12 days") + "\r\n", 0.1)
add("  " + ok("✓") + " sonarr      " + ok("running") + "  " + dim("Up 12 days") + "\r\n", 0.1)
add("  " + ok("✓") + " prowlarr    " + ok("running") + "  " + dim("Up 12 days") + "\r\n", 0.1)
add("\r\n", 0.25)

# Category 4: Disk
add("  " + bold("Disk Space") + "\r\n", 0.0)
add(sep(), 0.0)
add("  " + ok("✓") + " /media  75% used — " + ok("2.4 TB free") + "\r\n", 0.2)
add("\r\n", 0.2)

# Category 5: Media Paths
add("  " + bold("Media Paths") + "\r\n", 0.0)
add(sep(), 0.0)
add("  " + ok("✓") + " /media/movies  exists\r\n", 0.14)
add("  " + ok("✓") + " /media/tv      exists\r\n", 0.1)
add("\r\n", 0.2)

# Category 6: Indexers
add("  " + bold("Indexers") + "\r\n", 0.0)
add(sep(), 0.0)
add("  " + ok("✓") + " 12/12 indexers enabled and healthy\r\n", 0.2)
add("\r\n", 0.2)

# Category 7: VPN (issue!)
add("  " + bold("VPN") + "\r\n", 0.0)
add(sep(), 0.0)
add("  " + warn("⚠") + " Gluetun tunnel not detected — qBittorrent not bound to VPN interface\r\n", 0.2)
add("\r\n", 0.25)

# Summary
add(sep(), 0.0)
add("\r\n", 0.1)
add("  " + BOLD + "33/34 checks passed" + RESET + "  " + warn("1 issue detected") + "\r\n", 0.1)
add("\r\n", 0.1)
add("  " + warn("1.") + " VPN not detected — qBittorrent downloads may not be tunneled\r\n", 0.1)
add("\r\n", 0.1)
add("  " + gold("⚓") + " Run " + gold("admirarr doctor --fix") + " to auto-fix.\r\n", 0.1)
add("\r\n", 0.3)


# ─── SECTION 4: admirarr search ──────────────────────────────────────────────
type_cmd("admirarr search interstellar", pre_delay=1.5)

add(banner(), 0.3)
add("  " + bold("Prowlarr Search: interstellar") + "\r\n", 0.1)
add(sep(), 0.0)
add("\r\n", 0.5)

# Header
add("  " + dim("INDEXER          SEED    SIZE      TITLE") + "\r\n", 0.0)
add("  " + dim("─" * 68) + "\r\n", 0.05)

# Results
add("  " + ok("YTS") + "              " + ok("3,847") + "   " + dim("2.2 GB") + "   Interstellar.2014.2160p.UHD.BluRay.x265-B0MBARDiERS\r\n", 0.25)
add("  " + ok("1337x") + "            " + ok("1,204") + "   " + dim("56.3 GB") + "  Interstellar.2014.COMPLETE.UHD.BluRay-SURCODE\r\n", 0.12)
add("  " + ok("RARBG") + "            " + warn("892") + "    " + dim("14.7 GB") + "  Interstellar.2014.2160p.UHD.BluRay.DDP7.1.DoVi.x265\r\n", 0.12)
add("  " + ok("EZTV") + "             " + warn("441") + "    " + dim("3.8 GB") + "   Interstellar.2014.1080p.BluRay.x264.DTS-FGT\r\n", 0.1)
add("  " + ok("TorrentGalaxy") + "    " + dim("237") + "    " + dim("7.2 GB") + "   Interstellar.2014.4K.BluRay.HEVC.TrueHD.Atmos.7.1\r\n", 0.1)
add("  " + dim("Nyaa") + "             " + dim("89") + "     " + dim("1.4 GB") + "   Interstellar.2014.1080p.x265.10bit.mkv\r\n", 0.1)
add("\r\n", 0.25)
add("  " + dim("6 results — 12 indexers queried") + "\r\n", 0.1)
add("\r\n", 0.2)
add("  " + gold("⚓") + " Run " + gold("admirarr add-movie interstellar") + " to grab + auto-import.\r\n", 0.1)
add("\r\n", 0.8)

# Write the cast file
header = {
    "version": 2,
    "width": 82,
    "height": 42,
    "timestamp": 1743106800,
    "env": {"SHELL": "/bin/bash", "TERM": "xterm-256color"},
    "title": "Admirarr — Command your fleet"
}

output_path = sys.argv[1] if len(sys.argv) > 1 else "demo-full.cast"
with open(output_path, "w") as f:
    f.write(json.dumps(header) + "\n")
    for event in events:
        f.write(json.dumps(event) + "\n")

print(f"Generated {len(events)} events, duration={t:.1f}s -> {output_path}")
