package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/migrate"
	"github.com/maxtechera/admirarr/internal/setup"
	"github.com/maxtechera/admirarr/internal/ui"
	"github.com/maxtechera/admirarr/internal/vpn"
	"github.com/maxtechera/admirarr/internal/wire"
	"github.com/spf13/cobra"
)

var vpnCmd = &cobra.Command{
	Use:   "vpn",
	Short: "VPN connection status and management",
	Long:  "Show VPN connection status and Mullvad account info. Use subcommands for setup and management.",
	Run:   runVPN,
}

var vpnSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive Mullvad VPN setup",
	Long:  "Create or connect a Mullvad account, generate WireGuard keys, and register a device.",
	Run:   runVPNSetup,
}

var vpnStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "VPN connection and account status",
	Run:   runVPNStatus,
}

var vpnDevicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List registered Mullvad devices",
	Run:   runVPNDevices,
}

var vpnRotateCmd = &cobra.Command{
	Use:   "rotate",
	Short: "Rotate WireGuard keys and restart Gluetun",
	Run:   runVPNRotate,
}

func init() {
	rootCmd.AddCommand(vpnCmd)
	vpnCmd.AddCommand(vpnSetupCmd)
	vpnCmd.AddCommand(vpnStatusCmd)
	vpnCmd.AddCommand(vpnDevicesCmd)
	vpnCmd.AddCommand(vpnRotateCmd)
}

func runVPN(cmd *cobra.Command, args []string) {
	runVPNStatus(cmd, args)
}

func runVPNStatus(cmd *cobra.Command, args []string) {
	vpnStatus := wire.GetVPNStatus()

	type statusOut struct {
		Connected bool   `json:"connected"`
		IP        string `json:"ip,omitempty"`
		Country   string `json:"country,omitempty"`
		Provider  string `json:"provider,omitempty"`
		Account   string `json:"account,omitempty"`
		ExpiresAt string `json:"expires_at,omitempty"`
		Error     string `json:"error,omitempty"`
	}

	out := statusOut{
		Connected: vpnStatus.Connected,
		IP:        vpnStatus.IP,
		Country:   vpnStatus.Country,
		Provider:  config.Get().VPN.Provider,
		Account:   config.Get().VPN.Account,
	}

	if vpnStatus.Err != nil {
		out.Error = vpnStatus.Err.Error()
	}

	// Try to get account expiry if we have an account
	if out.Account != "" {
		client := vpn.NewClient()
		token, err := client.GetToken(out.Account)
		if err == nil {
			acct, err := client.GetAccountInfo(token.AccessToken)
			if err == nil {
				out.ExpiresAt = acct.ExpiresAt.Format(time.RFC3339)
			}
		}
	}

	ui.PrintOrJSON(out, func() {
		ui.PrintBanner()
		fmt.Println(ui.Bold("\n  VPN Status\n"))

		if vpnStatus.Err != nil {
			fmt.Printf("  %s VPN: %s\n", ui.Err("✗"), ui.Err(vpnStatus.Err.Error()))
		} else if vpnStatus.Connected {
			detail := ui.Ok("connected")
			if vpnStatus.IP != "" {
				detail += fmt.Sprintf("  IP: %s", vpnStatus.IP)
			}
			if vpnStatus.Country != "" {
				detail += fmt.Sprintf(" (%s)", vpnStatus.Country)
			}
			fmt.Printf("  %s VPN: %s\n", ui.Ok("✓"), detail)
		} else {
			fmt.Printf("  %s VPN: %s\n", ui.Err("✗"), ui.Err("disconnected"))
		}

		if out.Provider != "" {
			fmt.Printf("  %s Provider: %s\n", ui.Dim("•"), out.Provider)
		}
		if out.Account != "" {
			fmt.Printf("  %s Account: %s\n", ui.Dim("•"), vpn.FormatAccountNumber(out.Account))
		}
		if out.ExpiresAt != "" {
			t, err := time.Parse(time.RFC3339, out.ExpiresAt)
			if err == nil {
				remaining := time.Until(t)
				if remaining <= 0 {
					fmt.Printf("  %s Expired: %s\n", ui.Err("✗"), ui.Err(t.Format("2006-01-02")))
				} else if remaining < 7*24*time.Hour {
					fmt.Printf("  %s Expires: %s %s\n", ui.Warn("!"), t.Format("2006-01-02"), ui.Warn(fmt.Sprintf("(%d days)", int(remaining.Hours()/24))))
				} else {
					fmt.Printf("  %s Expires: %s\n", ui.Dim("•"), t.Format("2006-01-02"))
				}
			}
		}

		fmt.Println()
	})
}

func runVPNSetup(cmd *cobra.Command, args []string) {
	ui.PrintBanner()
	fmt.Println(ui.Bold("\n  Mullvad VPN Setup\n"))

	state := &setup.SetupState{
		VPNProvider: "mullvad",
		VPNType:     "wireguard",
		VPNCreds:    make(map[string]string),
		ComposeDir:  filepath.Dir(config.Get().ComposePath),
	}

	setup.MullvadAutoSetup(state)

	// Write updated .env if credentials were collected
	if len(state.VPNCreds) > 0 {
		envPath := filepath.Join(state.ComposeDir, ".env")
		opts := migrate.ComposeOpts{
			VPNProvider: "mullvad",
			VPNType:     "wireguard",
			VPNCreds:    state.VPNCreds,
		}
		envContent := migrate.GenerateEnvFile(opts, envPath)

		if err := os.MkdirAll(filepath.Dir(envPath), 0755); err != nil {
			fmt.Printf("  %s Cannot create directory: %v\n", ui.Err("✗"), err)
			return
		}
		if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
			fmt.Printf("  %s Cannot write .env: %v\n", ui.Err("✗"), err)
			return
		}
		fmt.Printf("\n  %s Written %s\n", ui.Ok("✓"), envPath)

		fmt.Printf("\n  %s Restart Gluetun to apply: %s\n", ui.Warn("!"), ui.Bold("admirarr restart gluetun"))
	}
}

func runVPNDevices(cmd *cobra.Command, args []string) {
	account := resolveAccount()
	if account == "" {
		if ui.IsJSON() {
			ui.PrintJSON([]struct{}{})
		} else {
			ui.PrintBanner()
			fmt.Println(ui.Bold("\n  Mullvad Devices\n"))
			fmt.Printf("  %s No Mullvad account configured\n", ui.Dim("—"))
			fmt.Printf("  %s Run %s to set up\n\n", ui.Dim("→"), ui.Bold("admirarr vpn setup"))
		}
		return
	}

	client := vpn.NewClient()
	token, err := client.GetToken(account)
	if err != nil {
		if ui.IsJSON() {
			ui.PrintJSON(map[string]string{"error": err.Error()})
		} else {
			fmt.Printf("  %s Cannot authenticate: %v\n", ui.Err("✗"), err)
		}
		return
	}

	devices, err := client.ListDevices(token.AccessToken)
	if err != nil {
		if ui.IsJSON() {
			ui.PrintJSON(map[string]string{"error": err.Error()})
		} else {
			fmt.Printf("  %s Cannot list devices: %v\n", ui.Err("✗"), err)
		}
		return
	}

	ui.PrintOrJSON(devices, func() {
		ui.PrintBanner()
		fmt.Println(ui.Bold("\n  Mullvad Devices\n"))
		if len(devices) == 0 {
			fmt.Printf("  %s\n", ui.Dim("No devices registered"))
		} else {
			for _, d := range devices {
				pubkeyShort := d.Pubkey
				if len(pubkeyShort) > 12 {
					pubkeyShort = pubkeyShort[:12] + "..."
				}
				fmt.Printf("  %-20s %s  %s\n", d.Name, d.IPv4Address, ui.Dim(pubkeyShort))
			}
		}
		fmt.Println()
	})
}

func runVPNRotate(cmd *cobra.Command, args []string) {
	account := resolveAccount()
	if account == "" {
		fmt.Printf("  %s No Mullvad account configured — run %s first\n", ui.Err("✗"), ui.Bold("admirarr vpn setup"))
		return
	}

	client := vpn.NewClient()
	token, err := client.GetToken(account)
	if err != nil {
		fmt.Printf("  %s Cannot authenticate: %v\n", ui.Err("✗"), err)
		return
	}

	// Generate new keys
	var privateKey, publicKey string
	err = ui.SpinWhile("Generating new WireGuard keys", func() error {
		var genErr error
		privateKey, publicKey, genErr = vpn.GenerateKeyPair()
		return genErr
	})
	if err != nil {
		return
	}

	// Register new device
	var device *vpn.Device
	err = ui.SpinWhile("Registering new device", func() error {
		var regErr error
		device, regErr = client.RegisterDevice(token.AccessToken, publicKey)
		return regErr
	})
	if err != nil {
		fmt.Printf("  %s Consider removing old devices with %s\n", ui.Warn("!"), ui.Bold("admirarr vpn devices"))
		return
	}

	// Update .env
	composeDir := filepath.Dir(config.Get().ComposePath)
	envPath := filepath.Join(composeDir, ".env")

	creds := map[string]string{
		"WIREGUARD_PRIVATE_KEY": privateKey,
		"WIREGUARD_ADDRESSES":  device.IPv4Address,
		"MULLVAD_ACCOUNT":      account,
	}
	opts := migrate.ComposeOpts{
		VPNProvider: "mullvad",
		VPNType:     "wireguard",
		VPNCreds:    creds,
	}
	envContent := migrate.GenerateEnvFile(opts, envPath)
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		fmt.Printf("  %s Cannot write .env: %v\n", ui.Err("✗"), err)
		return
	}
	fmt.Printf("  %s Updated %s\n", ui.Ok("✓"), envPath)

	// Restart Gluetun
	gluetunContainer := config.ContainerName("gluetun")
	err = ui.SpinWhile("Restarting Gluetun", func() error {
		return exec.Command("docker", "restart", gluetunContainer).Run()
	})
	if err != nil {
		return
	}

	fmt.Printf("\n  %s VPN keys rotated and Gluetun restarted\n", ui.Ok("✓"))
	fmt.Printf("  %s New device: %s\n", ui.Dim("•"), device.IPv4Address)
}

// resolveAccount returns the Mullvad account number from config or .env.
func resolveAccount() string {
	// Try config first
	if acct := config.Get().VPN.Account; acct != "" {
		return acct
	}
	// Fall back to .env file
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
