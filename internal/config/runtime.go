package config

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RuntimeInfo describes where a service is running.
type RuntimeInfo struct {
	Type  string // "docker", "native", "remote", "windows"
	Label string // human-readable, e.g. "docker (radarr)", "remote", "native", "windows"
}

// ServiceStatus holds the result of probing a service.
type ServiceStatus struct {
	Up        bool
	Host      string // resolved host where the service was found
	LatencyMs int64
	Runtime   RuntimeInfo
	Warnings  []string // misconfigurations detected during probing
}

// TypeWindows indicates a service running on the Windows host, only reachable from Windows.
const TypeWindows = "windows"

// ProbeAll probes all services with ports across all candidate hosts in parallel.
// On WSL, services not found from Linux are batch-probed from the Windows side.
// Updates in-memory config hosts to match where services were actually found.
func ProbeAll() map[string]ServiceStatus {
	names := AllServiceNames()
	dockerContainers := listDockerContainers()
	wsl := isWSL()
	result := make(map[string]ServiceStatus, len(names))
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Phase 1: Probe all services from WSL/Linux side in parallel
	for _, name := range names {
		def, ok := GetServiceDef(name)
		if !ok || def.Port == 0 {
			continue
		}
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			ss := probeServiceLinux(n, dockerContainers)

			// Warning: host mismatch (found somewhere other than configured)
			configured := Get().Services[n].Host
			if ss.Up && configured != "" && configured != ss.Host {
				ss.Warnings = append(ss.Warnings,
					fmt.Sprintf("Found at %s but configured as %s — update config or run admirarr setup", ss.Host, configured))
			}

			mu.Lock()
			result[n] = ss
			if ss.Up && ss.Host != ServiceHost(n) {
				SetServiceHost(n, ss.Host)
			}
			mu.Unlock()
		}(name)
	}
	wg.Wait()

	// Phase 2 (WSL only): Batch-probe all unreachable ports from Windows side.
	// One PowerShell call checks all ports at once using .NET sockets (~1-2s total).
	if wsl {
		var failedNames []string
		var failedPorts []int
		for _, name := range names {
			ss, ok := result[name]
			if ok && !ss.Up {
				failedNames = append(failedNames, name)
				failedPorts = append(failedPorts, ServicePort(name))
			}
		}

		if len(failedPorts) > 0 {
			t0 := time.Now()
			openPorts := windowsProbeBatch(failedPorts)
			ms := time.Since(t0).Milliseconds()

			for i, name := range failedNames {
				port := failedPorts[i]
				if openPorts[port] {
					result[name] = ServiceStatus{
						Up:        true,
						Host:      "localhost",
						LatencyMs: ms,
						Runtime:   RuntimeInfo{Type: TypeWindows, Label: "windows (localhost only)"},
						Warnings: []string{
							"Only reachable from Windows — Docker containers cannot reach this service. Migrate to Docker or bind to 0.0.0.0",
						},
					}
				}
			}
		}
	}

	// Phase 3: Detect port conflicts (multiple services sharing the same port and both up)
	portUsers := make(map[int][]string)
	for _, name := range names {
		ss, ok := result[name]
		if ok && ss.Up {
			port := ServicePort(name)
			portUsers[port] = append(portUsers[port], name)
		}
	}
	for port, users := range portUsers {
		if len(users) > 1 {
			for _, name := range users {
				ss := result[name]
				ss.Warnings = append(ss.Warnings,
					fmt.Sprintf("Port %d shared with %s — response may belong to another service", port, otherNames(users, name)))
				result[name] = ss
			}
		}
	}

	return result
}

// probeServiceLinux tries all candidate hosts for a service from the Linux/WSL side.
func probeServiceLinux(name string, dockerContainers map[string]bool) ServiceStatus {
	port := ServicePort(name)
	candidates := CandidateHosts(name)

	for _, host := range candidates {
		t0 := time.Now()
		if httpProbe(host, port) {
			ms := time.Since(t0).Milliseconds()
			rt := detectRuntime(name, host, dockerContainers)
			return ServiceStatus{
				Up:        true,
				Host:      host,
				LatencyMs: ms,
				Runtime:   rt,
			}
		}
	}

	// Not reachable from Linux side
	configuredHost := Get().Services[name].Host
	rt := detectRuntime(name, configuredHost, dockerContainers)
	return ServiceStatus{
		Up:      false,
		Host:    configuredHost,
		Runtime: rt,
	}
}

// httpProbe does a quick HTTP GET to check if a service is listening.
func httpProbe(host string, port int) bool {
	c := &http.Client{Timeout: 1 * time.Second}
	resp, err := c.Get(fmt.Sprintf("http://%s:%d/", host, port))
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode < 500
}

// windowsProbeBatch checks multiple ports on Windows localhost in a single PowerShell call.
// Uses .NET TcpClient for fast TCP probing (~1-2s total regardless of port count).
// Returns a set of ports that are open.
func windowsProbeBatch(ports []int) map[int]bool {
	ps := "/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"
	if _, err := os.Stat(ps); err != nil {
		return make(map[int]bool)
	}

	// Deduplicate ports
	seen := make(map[int]bool)
	var unique []string
	for _, p := range ports {
		if !seen[p] {
			seen[p] = true
			unique = append(unique, strconv.Itoa(p))
		}
	}

	// Single PowerShell call: try async TCP connect on each port with 500ms timeout.
	// Much faster than synchronous Connect() which blocks ~2s per closed port.
	script := fmt.Sprintf(
		`$ports = @(%s); foreach ($p in $ports) { `+
			`$tcp = New-Object System.Net.Sockets.TcpClient; `+
			`$ar = $tcp.BeginConnect('localhost', $p, $null, $null); `+
			`$ok = $ar.AsyncWaitHandle.WaitOne(500); `+
			`if ($ok -and $tcp.Connected) { Write-Output $p } `+
			`$tcp.Close() }`,
		strings.Join(unique, ","))

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	out, _ := exec.CommandContext(ctx, ps, "-NoProfile", "-Command", script).CombinedOutput()

	result := make(map[int]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if p, err := strconv.Atoi(line); err == nil {
			result[p] = true
		}
	}
	return result
}

// DetectRuntime determines how a single service is deployed using a fresh Docker check.
func DetectRuntime(name string) RuntimeInfo {
	host := ServiceHost(name)
	return detectRuntime(name, host, listDockerContainers())
}

// DetectAllRuntimes detects runtime info for all services with ports.
func DetectAllRuntimes() map[string]RuntimeInfo {
	names := AllServiceNames()
	result := make(map[string]RuntimeInfo, len(names))
	dockerContainers := listDockerContainers()

	for _, name := range names {
		def, ok := GetServiceDef(name)
		if !ok || def.Port == 0 {
			continue
		}
		host := ServiceHost(name)
		result[name] = detectRuntime(name, host, dockerContainers)
	}
	return result
}

// detectRuntime auto-detects deployment type:
//  1. Non-localhost host → remote
//  2. Docker container running → docker
//  3. Otherwise → native
func detectRuntime(name, host string, dockerContainers map[string]bool) RuntimeInfo {
	if host != "" && host != "localhost" && host != "127.0.0.1" {
		return RuntimeInfo{Type: TypeRemote, Label: "remote"}
	}

	container := ContainerName(name)
	svc := Get().Services[name]
	if svc.ContainerName != "" {
		container = svc.ContainerName
	}
	if container != "" && dockerContainers[container] {
		return RuntimeInfo{Type: TypeDocker, Label: fmt.Sprintf("docker (%s)", container)}
	}

	return RuntimeInfo{Type: TypeNative, Label: "native"}
}

// listDockerContainers returns a set of all running Docker container names.
func listDockerContainers() map[string]bool {
	out, err := exec.Command("docker", "ps", "--format", "{{.Names}}").Output()
	if err != nil {
		return make(map[string]bool)
	}
	result := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		name := strings.TrimSpace(line)
		if name != "" {
			result[name] = true
		}
	}
	return result
}

// isWSL returns true if running inside Windows Subsystem for Linux.
func isWSL() bool {
	_, err := os.Stat("/mnt/c/Windows")
	return err == nil
}

// otherNames returns a comma-separated list of names excluding the given one.
func otherNames(names []string, exclude string) string {
	var others []string
	for _, n := range names {
		if n != exclude {
			others = append(others, n)
		}
	}
	return strings.Join(others, ", ")
}
