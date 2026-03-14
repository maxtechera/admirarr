package setup

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/maxtechera/admirarr/internal/api"
	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/migrate"
	"github.com/maxtechera/admirarr/internal/ui"
	"github.com/maxtechera/admirarr/internal/vpn"
)

// checkNativeReachable tries api.CheckReachable first (uses config host),
// then falls back to checking on remoteHost if provided.
func checkNativeReachable(name, remoteHost string, port int) bool {
	if api.CheckReachable(name) {
		return true
	}
	if remoteHost == "" {
		return false
	}
	// Try the remote host IP directly
	c := &http.Client{Timeout: 3 * time.Second}
	url := fmt.Sprintf("http://%s:%d/ping", remoteHost, port)
	resp, err := c.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// DeployStack runs Phase 2: deploy missing services via Docker Compose.
func DeployStack(state *SetupState) StepResult {
	r := StepResult{Name: "Deploy Stack"}

	// Check Docker is available
	if err := exec.Command("docker", "version").Run(); err != nil {
		r.errf("Docker not found — install Docker or start Docker Desktop")
		return r
	}

	// Set global host in runtime config if detected
	if state.RemoteHost != "" {
		config.SetGlobalHost(state.RemoteHost)
	}

	// Triage services into detected, reachable (native), and missing
	var detected, reachable, missing []string
	for _, name := range state.SelectedServices {
		svc := state.Services[name]
		if svc == nil {
			continue
		}
		if svc.Detected {
			detected = append(detected, name)
		} else {
			// Check if reachable without a container (native/remote)
			def, _ := config.GetServiceDef(name)
			if def.Port > 0 && def.HasAPI && checkNativeReachable(name, state.RemoteHost, def.Port) {
				reachable = append(reachable, name)
				svc.Reachable = true
				// Update service host for remote services
				if state.RemoteHost != "" {
					svc.Host = state.RemoteHost
				}
			} else {
				missing = append(missing, name)
			}
		}
	}

	// Report detected services
	for _, name := range detected {
		fmt.Printf("  %s %-15s %s\n", ui.Ok("✓"), name, ui.Dim("detected"))
		r.pass()
	}

	// Handle reachable-but-not-Docker services (Scenario E)
	if len(reachable) > 0 {
		fmt.Printf("\n  Detected %d services running outside Docker:\n", len(reachable))
		for _, name := range reachable {
			def, _ := config.GetServiceDef(name)
			fmt.Printf("    %s %-15s %s\n", ui.Dim("•"), name, ui.Dim(fmt.Sprintf("native, :%d reachable", def.Port)))
		}

		choice := "keep"
		if state.AutoMode {
			// Auto mode: keep existing native deployments
		} else {
			form := huh.NewForm(huh.NewGroup(
				huh.NewSelect[string]().
					Title("What would you like to do?").
					Options(
						huh.NewOption("Keep existing deployment (configure as-is)", "keep"),
						huh.NewOption("Migrate to Docker (deploy containers)", "migrate"),
					).
					Value(&choice),
			))
			if err := form.Run(); err != nil {
				choice = "keep"
			}
		}

		if choice == "migrate" {
			fmt.Printf("\n  %s Stop native services before deploying Docker containers:\n", ui.Warn("!"))
			for _, name := range reachable {
				fmt.Printf("    sudo systemctl stop %s\n", name)
			}
			fmt.Println()
			missing = append(missing, reachable...)
		} else {
			for _, name := range reachable {
				fmt.Printf("  %s %-15s %s\n", ui.Ok("✓"), name, ui.Dim("native (kept)"))
				r.pass()
			}
		}
	}

	// Early exit if nothing to deploy
	if len(missing) == 0 {
		if len(detected) > 0 || len(reachable) > 0 {
			fmt.Printf("\n  %s All services detected, nothing to deploy\n", ui.Ok("✓"))
		}
		return r
	}

	// VPN prompt (if gluetun or qbittorrent in missing)
	needsVPN := false
	for _, name := range missing {
		if name == "gluetun" || name == "qbittorrent" {
			needsVPN = true
			break
		}
	}
	if needsVPN {
		promptVPN(state)
	}

	// Filter out any remote/native services from the deploy list.
	// A service might land in `missing` because it was unreachable during probe
	// but is configured as remote — deploying it in Docker would cause port conflicts.
	remoteInMissing := config.RemoteServices(missing)
	missing = config.DockerOnlyServices(missing)
	if len(remoteInMissing) > 0 {
		for _, name := range remoteInMissing {
			fmt.Printf("  %s %-15s %s\n", ui.Dim("—"), name, ui.Dim("remote (skipped from Docker deploy)"))
		}
	}

	// Collect all remote/native services for compose header documentation
	var remoteAll []string
	for _, name := range state.SelectedServices {
		svc := state.Services[name]
		if svc == nil {
			continue
		}
		// Services that are reachable but not Docker, or explicitly remote in config
		cfgSvc := config.Get().Services[name]
		if cfgSvc.Type == config.TypeRemote || (svc.Reachable && !svc.IsDocker && svc.Host != "localhost" && svc.Host != "") {
			remoteAll = append(remoteAll, name)
		}
	}

	// Early exit if nothing to deploy (recheck after filtering)
	if len(missing) == 0 {
		fmt.Printf("\n  %s No Docker services to deploy\n", ui.Ok("✓"))
		return r
	}

	// Generate compose
	composeOpts := migrate.ComposeOpts{
		DataDir:        state.DataPath,
		ConfigDir:      state.ConfigDir,
		TZ:             state.Timezone,
		PUID:           fmt.Sprintf("%d", os.Getuid()),
		PGID:           fmt.Sprintf("%d", os.Getgid()),
		VPNProvider:    state.VPNProvider,
		VPNType:        state.VPNType,
		VPNCreds:       state.VPNCreds,
		RemoteServices: remoteAll,
	}

	composeContent := migrate.GenerateCompose(missing, composeOpts)

	// Write files
	composePath := filepath.Join(state.ComposeDir, "docker-compose.yml")
	envPath := filepath.Join(state.ComposeDir, ".env")

	// Generate .env with merge (preserves existing VPN creds, custom vars)
	envContent := migrate.GenerateEnvFile(composeOpts, envPath)

	if state.wouldFix(&r, "Write docker-compose.yml to %s", composePath) {
		state.wouldFix(&r, "Write .env to %s", envPath)
		state.wouldFix(&r, "Run docker compose up for %d services", len(missing))
		return r
	}

	if err := os.MkdirAll(state.ComposeDir, 0755); err != nil {
		r.errf("cannot create compose directory: %v", err)
		return r
	}

	// Backup existing compose file
	if _, err := os.Stat(composePath); err == nil {
		backup := composePath + fmt.Sprintf(".bak.%s", time.Now().Format("20060102-150405"))
		if err := os.Rename(composePath, backup); err == nil {
			fmt.Printf("  %s Backed up existing compose to %s\n", ui.Dim("→"), filepath.Base(backup))
		}
	}

	if err := os.WriteFile(composePath, []byte(composeContent), 0644); err != nil {
		r.errf("cannot write docker-compose.yml: %v", err)
		return r
	}
	fmt.Printf("  %s Written %s\n", ui.Ok("✓"), composePath)

	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		r.errf("cannot write .env: %v", err)
		return r
	}
	fmt.Printf("  %s Written %s\n", ui.Ok("✓"), envPath)

	// Docker compose up
	var deployErr error
	ui.SpinWhile("Deploying containers", func() error {
		// Try compose V2 first, fall back to V1
		cmd := exec.Command("docker", "compose", "-f", composePath, "up", "-d")
		cmd.Dir = state.ComposeDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			// Try docker-compose V1
			cmd2 := exec.Command("docker-compose", "-f", composePath, "up", "-d")
			cmd2.Dir = state.ComposeDir
			out2, err2 := cmd2.CombinedOutput()
			if err2 != nil {
				deployErr = fmt.Errorf("%s\n%s", strings.TrimSpace(string(out)), strings.TrimSpace(string(out2)))
				return deployErr
			}
		} else {
			_ = out
		}
		return nil
	})

	if deployErr != nil {
		r.errf("docker compose up failed: %v", deployErr)
		return r
	}

	// Wait for containers to come up (60s max, poll every 3s)
	ui.SpinWhile("Waiting for containers to start", func() error {
		deadline := time.Now().Add(60 * time.Second)
		for time.Now().Before(deadline) {
			allUp := true
			for _, name := range missing {
				container := config.ContainerName(name)
				out, err := exec.Command("docker", "ps", "--filter",
					fmt.Sprintf("name=%s", container), "--format", "{{.Status}}").Output()
				if err != nil || !strings.HasPrefix(strings.TrimSpace(string(out)), "Up") {
					allUp = false
					break
				}
			}
			if allUp {
				return nil
			}
			time.Sleep(3 * time.Second)
		}
		return nil // timeout is a warning, not an error
	})

	// Post-deploy stabilization — services need time to generate config files
	ui.SpinWhile("Waiting for services to stabilize", func() error {
		time.Sleep(5 * time.Second)
		return nil
	})

	// Re-detect services
	out, err := exec.Command("docker", "ps", "-a", "--format", "{{.Names}}").Output()
	if err == nil {
		containers := strings.ToLower(string(out))
		for _, name := range missing {
			container := config.ContainerName(name)
			svc := state.Services[name]
			if svc == nil {
				continue
			}
			if strings.Contains(containers, strings.ToLower(container)) {
				svc.Detected = true
				svc.IsDocker = true
				fmt.Printf("  %s %-15s %s\n", ui.Ok("✓"), name, ui.GoldText("deployed"))
				r.fix()
			} else {
				fmt.Printf("  %s %-15s %s\n", ui.Err("✗"), name, ui.Err("container not found after deploy"))
				r.errf("%s: container did not start", name)
			}
		}
	}

	return r
}

// vpnNeedsAddresses returns true for providers that require WIREGUARD_ADDRESSES.
func vpnNeedsAddresses(provider string) bool {
	return provider == "mullvad" || provider == "surfshark"
}

func promptVPN(state *SetupState) {
	if state.AutoMode {
		state.VPNProvider = "mullvad"
		state.VPNType = "wireguard"
		fmt.Printf("  %s VPN: %s (%s) %s\n", ui.Ok("✓"), state.VPNProvider, state.VPNType, ui.Dim("(auto)"))
		mullvadAutoSetupHeadless(state)
		return
	}

	var provider string
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("VPN Provider").
			Options(
				huh.NewOption("Mullvad", "mullvad"),
				huh.NewOption("NordVPN", "nordvpn"),
				huh.NewOption("Surfshark", "surfshark"),
				huh.NewOption("ProtonVPN", "protonvpn"),
				huh.NewOption("Private Internet Access", "private internet access"),
				huh.NewOption("Custom", "custom"),
			).
			Value(&provider),
	))
	if err := form.Run(); err != nil {
		provider = "mullvad"
	}
	state.VPNProvider = provider

	var vpnType string
	form = huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("VPN Type").
			Options(
				huh.NewOption("WireGuard (recommended)", "wireguard"),
				huh.NewOption("OpenVPN", "openvpn"),
			).
			Value(&vpnType),
	))
	if err := form.Run(); err != nil {
		vpnType = "wireguard"
	}
	state.VPNType = vpnType

	fmt.Printf("  %s VPN: %s (%s)\n", ui.Ok("✓"), state.VPNProvider, state.VPNType)

	// Mullvad + WireGuard: automated setup flow
	if state.VPNProvider == "mullvad" && state.VPNType == "wireguard" {
		mullvadAutoSetup(state)
		return
	}

	// Other providers: manual credential collection
	promptVPNCreds(state)
}

func promptVPNCreds(state *SetupState) {
	if state.VPNCreds == nil {
		state.VPNCreds = make(map[string]string)
	}
	if state.VPNType == "wireguard" {
		var privateKey string
		form := huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("WireGuard Private Key").
				Description("From your VPN provider's WireGuard config").
				Value(&privateKey),
		))
		if err := form.Run(); err == nil && privateKey != "" {
			state.VPNCreds["WIREGUARD_PRIVATE_KEY"] = privateKey
		}

		if vpnNeedsAddresses(state.VPNProvider) {
			var addresses string
			form = huh.NewForm(huh.NewGroup(
				huh.NewInput().
					Title("WireGuard Addresses").
					Description("Interface address (e.g. 10.64.0.1/32)").
					Value(&addresses),
			))
			if err := form.Run(); err == nil && addresses != "" {
				state.VPNCreds["WIREGUARD_ADDRESSES"] = addresses
			}
		}
	} else {
		// OpenVPN
		var username string
		form := huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("OpenVPN Username").
				Value(&username),
		))
		if err := form.Run(); err == nil && username != "" {
			state.VPNCreds["OPENVPN_USER"] = username
		}

		var password string
		form = huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("OpenVPN Password").
				EchoMode(huh.EchoModePassword).
				Value(&password),
		))
		if err := form.Run(); err == nil && password != "" {
			state.VPNCreds["OPENVPN_PASSWORD"] = password
		}
	}

	// Optional: server countries
	var countries string
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Server Countries (optional)").
			Description("Comma-separated, e.g. US,NL — leave empty for auto").
			Value(&countries),
	))
	if err := form.Run(); err == nil && countries != "" {
		state.VPNCreds["SERVER_COUNTRIES"] = countries
	}

	if len(state.VPNCreds) > 0 {
		fmt.Printf("  %s VPN credentials collected (%d vars)\n", ui.Ok("✓"), len(state.VPNCreds))
	}
}

// MullvadAutoSetup runs the automated Mullvad VPN setup flow.
// Exported so cmd/vpn.go can reuse it for `admirarr vpn setup`.
func MullvadAutoSetup(state *SetupState) {
	mullvadAutoSetup(state)
}

func mullvadAutoSetup(state *SetupState) {
	if state.VPNCreds == nil {
		state.VPNCreds = make(map[string]string)
	}

	client := vpn.NewClient()

	// Step 1: New account or existing?
	var choice string
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Mullvad Account").
			Options(
				huh.NewOption("Create a new account (free, pay later)", "new"),
				huh.NewOption("Enter existing account number", "existing"),
			).
			Value(&choice),
	))
	if err := form.Run(); err != nil {
		choice = "new"
	}

	var accountNumber string
	var token *vpn.AuthToken

	if choice == "existing" {
		// Existing account: prompt for number and validate
		var acctNum string
		form = huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("Mullvad Account Number").
				Description("16-digit number (e.g. 1234567890123456)").
				Value(&acctNum),
		))
		if err := form.Run(); err != nil || acctNum == "" {
			fmt.Printf("  %s Skipped — configure later with %s\n", ui.Warn("!"), ui.Bold("admirarr vpn setup"))
			return
		}
		// Strip spaces from account number
		acctNum = strings.ReplaceAll(acctNum, " ", "")

		var tokenErr error
		ui.SpinWhile("Validating account", func() error {
			var err error
			token, err = client.GetToken(acctNum)
			if err != nil {
				tokenErr = err
				return err
			}
			return nil
		})
		if tokenErr != nil {
			fmt.Printf("  %s Invalid account: %v\n", ui.Err("✗"), tokenErr)
			fmt.Printf("  %s Configure later with %s\n", ui.Warn("!"), ui.Bold("admirarr vpn setup"))
			return
		}
		accountNumber = acctNum
	} else {
		// Create new account
		var acct *vpn.Account
		var createErr error
		ui.SpinWhile("Creating Mullvad account", func() error {
			var err error
			acct, err = client.CreateAccount()
			if err != nil {
				createErr = err
				return err
			}
			return nil
		})
		if createErr != nil {
			fmt.Printf("  %s Cannot create account: %v\n", ui.Err("✗"), createErr)
			fmt.Printf("  %s Configure later with %s\n", ui.Warn("!"), ui.Bold("admirarr vpn setup"))
			return
		}
		accountNumber = acct.Number
		fmt.Printf("  %s Account created: %s\n", ui.Ok("✓"), ui.Bold(vpn.FormatAccountNumber(accountNumber)))

		// Get token for the new account
		var tokenErr error
		ui.SpinWhile("Authenticating", func() error {
			var err error
			token, err = client.GetToken(accountNumber)
			if err != nil {
				tokenErr = err
				return err
			}
			return nil
		})
		if tokenErr != nil {
			fmt.Printf("  %s Auth failed: %v\n", ui.Err("✗"), tokenErr)
			fmt.Printf("  %s Configure later with %s\n", ui.Warn("!"), ui.Bold("admirarr vpn setup"))
			return
		}
	}

	// Step 2: Generate WireGuard keys
	var privateKey, publicKey string
	var keyErr error
	ui.SpinWhile("Generating WireGuard keys", func() error {
		var err error
		privateKey, publicKey, err = vpn.GenerateKeyPair()
		if err != nil {
			keyErr = err
			return err
		}
		return nil
	})
	if keyErr != nil {
		fmt.Printf("  %s Key generation failed: %v\n", ui.Err("✗"), keyErr)
		return
	}

	// Step 3: Register device
	var device *vpn.Device
	var regErr error
	ui.SpinWhile("Registering device with Mullvad", func() error {
		var err error
		device, err = client.RegisterDevice(token.AccessToken, publicKey)
		if err != nil {
			regErr = err
			return err
		}
		return nil
	})

	// Handle max devices: list and offer removal
	if errors.Is(regErr, vpn.ErrMaxDevices) {
		fmt.Printf("  %s Maximum devices reached (5)\n", ui.Warn("!"))

		devices, listErr := client.ListDevices(token.AccessToken)
		if listErr != nil {
			fmt.Printf("  %s Cannot list devices: %v\n", ui.Err("✗"), listErr)
			return
		}

		// Show devices and let user pick one to remove
		var options []huh.Option[string]
		for _, d := range devices {
			label := fmt.Sprintf("%s — %s (%s)", d.Name, d.Pubkey[:8]+"...", d.IPv4Address)
			options = append(options, huh.NewOption(label, d.ID))
		}
		options = append(options, huh.NewOption("Cancel (configure later)", "cancel"))

		var removeID string
		form = huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title("Remove a device to make room").
				Options(options...).
				Value(&removeID),
		))
		if err := form.Run(); err != nil || removeID == "cancel" {
			fmt.Printf("  %s Configure later with %s\n", ui.Warn("!"), ui.Bold("admirarr vpn setup"))
			return
		}

		if err := client.RemoveDevice(token.AccessToken, removeID); err != nil {
			fmt.Printf("  %s Cannot remove device: %v\n", ui.Err("✗"), err)
			return
		}
		fmt.Printf("  %s Device removed\n", ui.Ok("✓"))

		// Retry registration
		ui.SpinWhile("Registering device with Mullvad", func() error {
			var err error
			device, err = client.RegisterDevice(token.AccessToken, publicKey)
			if err != nil {
				regErr = err
				return err
			}
			regErr = nil
			return nil
		})
	}

	if regErr != nil {
		fmt.Printf("  %s Device registration failed: %v\n", ui.Err("✗"), regErr)
		fmt.Printf("  %s Configure later with %s\n", ui.Warn("!"), ui.Bold("admirarr vpn setup"))
		return
	}
	fmt.Printf("  %s Device registered: %s\n", ui.Ok("✓"), device.IPv4Address)

	// Step 4: Populate credentials
	state.VPNCreds["WIREGUARD_PRIVATE_KEY"] = privateKey
	state.VPNCreds["WIREGUARD_ADDRESSES"] = device.IPv4Address
	state.VPNCreds["MULLVAD_ACCOUNT"] = accountNumber

	fmt.Printf("\n  %s VPN credentials ready (%d vars written to .env)\n", ui.Ok("✓"), len(state.VPNCreds))

	// Payment reminder for new accounts
	if choice == "new" {
		fmt.Printf("\n  %s Account has no time — add payment to activate:\n", ui.Warn("!"))
		fmt.Printf("    → https://mullvad.net/account/login\n")
		fmt.Printf("    Account number: %s\n", ui.Bold(vpn.FormatAccountNumber(accountNumber)))
	}
}

// mullvadAutoSetupHeadless runs the Mullvad setup flow non-interactively.
// Creates a new account, generates keys, registers a device — no prompts.
// Used by `admirarr setup --auto`.
func mullvadAutoSetupHeadless(state *SetupState) {
	if state.VPNCreds == nil {
		state.VPNCreds = make(map[string]string)
	}

	// Check if credentials already exist in .env
	if state.ComposeDir != "" {
		envPath := filepath.Join(state.ComposeDir, ".env")
		if data, err := os.ReadFile(envPath); err == nil {
			content := string(data)
			if strings.Contains(content, "WIREGUARD_PRIVATE_KEY=") {
				// Extract existing creds to state so they flow through
				for _, line := range strings.Split(content, "\n") {
					for _, key := range []string{"WIREGUARD_PRIVATE_KEY", "WIREGUARD_ADDRESSES", "MULLVAD_ACCOUNT"} {
						if strings.HasPrefix(line, key+"=") {
							state.VPNCreds[key] = strings.TrimPrefix(line, key+"=")
						}
					}
				}
				if state.VPNCreds["WIREGUARD_PRIVATE_KEY"] != "" {
					fmt.Printf("  %s VPN credentials already configured\n", ui.Ok("✓"))
					return
				}
			}
		}
	}

	client := vpn.NewClient()

	// Create account
	var acct *vpn.Account
	var createErr error
	ui.SpinWhile("Creating Mullvad account", func() error {
		var err error
		acct, err = client.CreateAccount()
		if err != nil {
			createErr = err
			return err
		}
		return nil
	})
	if createErr != nil {
		fmt.Printf("  %s Cannot create Mullvad account: %v\n", ui.Err("✗"), createErr)
		fmt.Printf("  %s Run %s to configure manually\n", ui.Warn("!"), ui.Bold("admirarr vpn setup"))
		return
	}
	fmt.Printf("  %s Account created: %s\n", ui.Ok("✓"), ui.Bold(vpn.FormatAccountNumber(acct.Number)))

	// Get token
	var token *vpn.AuthToken
	var tokenErr error
	ui.SpinWhile("Authenticating", func() error {
		var err error
		token, err = client.GetToken(acct.Number)
		if err != nil {
			tokenErr = err
			return err
		}
		return nil
	})
	if tokenErr != nil {
		fmt.Printf("  %s Auth failed: %v\n", ui.Err("✗"), tokenErr)
		return
	}

	// Generate keys
	var privateKey, publicKey string
	var keyErr error
	ui.SpinWhile("Generating WireGuard keys", func() error {
		var err error
		privateKey, publicKey, err = vpn.GenerateKeyPair()
		if err != nil {
			keyErr = err
			return err
		}
		return nil
	})
	if keyErr != nil {
		fmt.Printf("  %s Key generation failed: %v\n", ui.Err("✗"), keyErr)
		return
	}

	// Register device
	var device *vpn.Device
	var regErr error
	ui.SpinWhile("Registering device with Mullvad", func() error {
		var err error
		device, err = client.RegisterDevice(token.AccessToken, publicKey)
		if err != nil {
			regErr = err
			return err
		}
		return nil
	})

	// Auto mode: if max devices, remove the oldest one automatically
	if errors.Is(regErr, vpn.ErrMaxDevices) {
		fmt.Printf("  %s Max devices reached — removing oldest device\n", ui.Warn("!"))
		devices, listErr := client.ListDevices(token.AccessToken)
		if listErr != nil || len(devices) == 0 {
			fmt.Printf("  %s Cannot resolve max devices: %v\n", ui.Err("✗"), listErr)
			return
		}
		// Remove the first device (oldest)
		if err := client.RemoveDevice(token.AccessToken, devices[0].ID); err != nil {
			fmt.Printf("  %s Cannot remove device: %v\n", ui.Err("✗"), err)
			return
		}
		fmt.Printf("  %s Removed device: %s\n", ui.Ok("✓"), devices[0].Name)

		// Retry
		ui.SpinWhile("Registering device with Mullvad", func() error {
			var err error
			device, err = client.RegisterDevice(token.AccessToken, publicKey)
			if err != nil {
				regErr = err
				return err
			}
			regErr = nil
			return nil
		})
	}

	if regErr != nil {
		fmt.Printf("  %s Device registration failed: %v\n", ui.Err("✗"), regErr)
		return
	}
	fmt.Printf("  %s Device registered: %s\n", ui.Ok("✓"), device.IPv4Address)

	// Populate credentials
	state.VPNCreds["WIREGUARD_PRIVATE_KEY"] = privateKey
	state.VPNCreds["WIREGUARD_ADDRESSES"] = device.IPv4Address
	state.VPNCreds["MULLVAD_ACCOUNT"] = acct.Number

	fmt.Printf("  %s VPN credentials ready (%d vars)\n", ui.Ok("✓"), len(state.VPNCreds))
	fmt.Printf("\n  %s Account has no time — add payment to activate:\n", ui.Warn("!"))
	fmt.Printf("    → https://mullvad.net/account/login\n")
	fmt.Printf("    Account number: %s\n", ui.Bold(vpn.FormatAccountNumber(acct.Number)))
}
