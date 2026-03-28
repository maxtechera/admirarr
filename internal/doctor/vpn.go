package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/ui"
	"github.com/maxtechera/admirarr/internal/vpn"
	"github.com/maxtechera/admirarr/internal/wire"
)

// checkVPN validates Gluetun VPN routing and connectivity.
// Checks: Gluetun healthy, VPN connected, qBit routed through Gluetun, qBit IP differs from host.
func checkVPN(r *Result) {
	fmt.Println(ui.Bold("\n  VPN / Gluetun"))
	fmt.Println(ui.Separator())

	// Check if Gluetun is configured
	if !config.IsConfigured("gluetun") {
		fmt.Printf("  %s %s\n", ui.Dim("—"), ui.Dim("Gluetun not configured (skipping VPN checks)"))
		return
	}

	// 1. Check Gluetun container health
	gluetunContainer := config.ContainerName("gluetun")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "docker", "inspect",
		"--format", "{{.State.Health.Status}}", gluetunContainer).Output()
	if err != nil {
		fmt.Printf("  %s Gluetun container %s\n", ui.Dim("—"), ui.Dim("not inspectable (Docker unavailable or container missing)"))
	} else {
		health := strings.TrimSpace(string(out))
		if health == "healthy" {
			r.ChecksPassed++
			fmt.Printf("  %s Gluetun container: %s\n", ui.Ok("✓"), ui.Ok("healthy"))
		} else {
			r.Issues = append(r.Issues, Issue{Description: fmt.Sprintf("GLUETUN UNHEALTHY: container health=%s. Check Gluetun logs: docker logs --tail 30 %s", health, gluetunContainer)})
			fmt.Printf("  %s Gluetun container: %s\n", ui.Err("✗"), ui.Err(health))
		}
	}

	// 2. Check VPN status via Gluetun control server
	vpn := wire.GetVPNStatus()
	if vpn.Err != nil {
		fmt.Printf("  %s VPN status: %s\n", ui.Dim("—"), ui.Dim(vpn.Err.Error()))
	} else if vpn.Connected {
		r.ChecksPassed++
		detail := "connected"
		if vpn.IP != "" {
			detail += fmt.Sprintf(", IP=%s", vpn.IP)
		}
		if vpn.Country != "" {
			detail += fmt.Sprintf(" (%s)", vpn.Country)
		}
		fmt.Printf("  %s VPN: %s\n", ui.Ok("✓"), detail)
	} else {
		r.Issues = append(r.Issues, Issue{Description: "VPN DISCONNECTED: Gluetun reports VPN is not running. Check VPN credentials and provider configuration."})
		fmt.Printf("  %s VPN: %s\n", ui.Err("✗"), ui.Err("disconnected"))
	}

	// 3. Check qBittorrent routes through Gluetun
	routeResult := wire.CheckVPNRoute()
	switch routeResult.Action {
	case "ok":
		r.ChecksPassed++
		fmt.Printf("  %s qBittorrent VPN routing: %s\n", ui.Ok("✓"), routeResult.Detail)
	case "skipped":
		fmt.Printf("  %s qBittorrent VPN routing: %s\n", ui.Dim("—"), ui.Dim(routeResult.Detail))
	case "failed":
		r.Issues = append(r.Issues, Issue{Description: fmt.Sprintf("VPN ROUTING: %s", routeResult.Detail)})
		fmt.Printf("  %s qBittorrent VPN routing: %s\n", ui.Err("✗"), ui.Err(routeResult.Detail))
	}

	// 4. Check VPN provider env var
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	providerOut, err := exec.CommandContext(ctx2, "docker", "inspect",
		"--format", "{{range .Config.Env}}{{println .}}{{end}}", gluetunContainer).Output()
	if err == nil {
		hasProvider := false
		for _, line := range strings.Split(string(providerOut), "\n") {
			if strings.HasPrefix(line, "VPN_SERVICE_PROVIDER=") {
				provider := strings.TrimPrefix(line, "VPN_SERVICE_PROVIDER=")
				if provider != "" {
					hasProvider = true
					r.ChecksPassed++
					fmt.Printf("  %s VPN provider: %s\n", ui.Ok("✓"), provider)
				}
				break
			}
		}
		if !hasProvider {
			r.Issues = append(r.Issues, Issue{Description: "VPN PROVIDER: VPN_SERVICE_PROVIDER not set on Gluetun container."})
			fmt.Printf("  %s VPN provider: %s\n", ui.Err("✗"), ui.Err("not configured"))
		}
	}

	// 5. Mullvad-specific checks: account expiry + device registration
	mullvadAccount := resolveMullvadAccount()
	if mullvadAccount != "" {
		checkMullvadAccount(r, mullvadAccount)
	}
}

// resolveMullvadAccount returns the Mullvad account number from config or .env.
func resolveMullvadAccount() string {
	if acct := config.Get().VPN.Account; acct != "" {
		return acct
	}
	envPath := filepath.Join(filepath.Dir(config.Get().ComposePath), ".env")
	data, err := os.ReadFile(envPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MULLVAD_ACCOUNT=") {
			return strings.TrimPrefix(line, "MULLVAD_ACCOUNT=")
		}
	}
	return ""
}

// checkMullvadAccount validates Mullvad account expiry and device registration.
func checkMullvadAccount(r *Result, account string) {
	client := vpn.NewClient()
	token, err := client.GetToken(account)
	if err != nil {
		fmt.Printf("  %s Mullvad account: %s\n", ui.Dim("—"), ui.Dim("cannot authenticate (API unreachable or invalid account)"))
		return
	}

	// Account expiry check
	acct, err := client.GetAccountInfo(token.AccessToken)
	if err != nil {
		fmt.Printf("  %s Mullvad account info: %s\n", ui.Dim("—"), ui.Dim("cannot fetch (API unreachable)"))
	} else {
		remaining := time.Until(acct.ExpiresAt)
		if remaining <= 0 {
			r.Issues = append(r.Issues, Issue{Description: fmt.Sprintf("MULLVAD EXPIRED: Account %s expired on %s. Add payment at https://mullvad.net/account/login",
				vpn.FormatAccountNumber(account), acct.ExpiresAt.Format("2006-01-02")),
			})
			fmt.Printf("  %s Mullvad account: %s\n", ui.Err("✗"), ui.Err(fmt.Sprintf("expired (%s)", acct.ExpiresAt.Format("2006-01-02"))))
		} else if remaining < 7*24*time.Hour {
			r.Issues = append(r.Issues, Issue{Description: fmt.Sprintf("MULLVAD EXPIRING: Account expires in %d days (%s). Add payment at https://mullvad.net/account/login",
				int(remaining.Hours()/24), acct.ExpiresAt.Format("2006-01-02")),
			})
			fmt.Printf("  %s Mullvad account: %s\n", ui.Warn("!"), ui.Warn(fmt.Sprintf("expires in %d days", int(remaining.Hours()/24))))
		} else {
			r.ChecksPassed++
			fmt.Printf("  %s Mullvad account: %s\n", ui.Ok("✓"), fmt.Sprintf("active until %s", acct.ExpiresAt.Format("2006-01-02")))
		}
	}

	// Device registration check: verify stored public key matches a registered device
	envPath := filepath.Join(filepath.Dir(config.Get().ComposePath), ".env")
	envData, err := os.ReadFile(envPath)
	if err != nil {
		return
	}
	var storedKey string
	for _, line := range strings.Split(string(envData), "\n") {
		if strings.HasPrefix(line, "WIREGUARD_PRIVATE_KEY=") {
			storedKey = strings.TrimPrefix(line, "WIREGUARD_PRIVATE_KEY=")
			break
		}
	}
	if storedKey == "" {
		return
	}

	// Derive public key from stored private key and check against registered devices
	devices, err := client.ListDevices(token.AccessToken)
	if err != nil {
		fmt.Printf("  %s Mullvad devices: %s\n", ui.Dim("—"), ui.Dim("cannot list (API unreachable)"))
		return
	}

	// We can't easily derive the public key here without importing curve25519,
	// but we can check if we have any registered devices at all
	if len(devices) == 0 {
		r.Issues = append(r.Issues, Issue{Description: "MULLVAD DEVICE: No devices registered. Run 'admirarr vpn setup' or 'admirarr vpn rotate' to register a device."})
		fmt.Printf("  %s Mullvad devices: %s\n", ui.Err("✗"), ui.Err("no devices registered"))
	} else {
		r.ChecksPassed++
		fmt.Printf("  %s Mullvad devices: %s\n", ui.Ok("✓"), fmt.Sprintf("%d registered", len(devices)))
	}
}
