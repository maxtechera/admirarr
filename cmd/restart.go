package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/maxtechera/admirarr/internal/ui"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart <service>",
	Short: "Restart a service",
	Long: `Restart a service (Windows service or Docker container).

Docker containers (restarted via docker restart):
  seerr, bazarr, organizr, flaresolverr

Windows services (restarted via PowerShell Restart-Service):
  sonarr, radarr, prowlarr, plex, tautulli`,
	Example: "  admirarr restart sonarr\n  admirarr restart seerr",
	Args:    cobra.ExactArgs(1),
	Run:     runRestart,
}

func init() {
	rootCmd.AddCommand(restartCmd)
}

var winServiceNames = map[string]string{
	"sonarr":   "Sonarr",
	"radarr":   "Radarr",
	"prowlarr": "Prowlarr",
	"plex":     "Plex Media Server",
	"tautulli": "Tautulli",
}

var dockerSvcs = map[string]bool{
	"seerr": true, "bazarr": true, "organizr": true, "flaresolverr": true,
}

func runRestart(cmd *cobra.Command, args []string) {
	service := strings.ToLower(args[0])

	if dockerSvcs[service] {
		fmt.Printf("  Restarting %s...\n", service)
		out, err := exec.Command("docker", "restart", service).CombinedOutput()
		if err != nil {
			fmt.Printf("  %s %s\n", ui.Err("Failed:"), strings.TrimSpace(string(out)))
		} else {
			fmt.Printf("  %s Restarted %s\n", ui.Ok("●"), service)
		}
	} else if winSvc, ok := winServiceNames[service]; ok {
		fmt.Printf("  Restarting %s (requires admin)...\n", service)
		psCmd := fmt.Sprintf(`Restart-Service '%s' -Force`, winSvc)
		c := exec.Command("/mnt/c/Windows/System32/cmd.exe", "/c",
			fmt.Sprintf(`powershell -Command "%s"`, psCmd))
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		_ = c.Run()
		fmt.Printf("  %s Restart command sent — verify with: admirarr status\n", ui.Ok("●"))
	} else {
		fmt.Printf("  %s\n", ui.Err(fmt.Sprintf("Unknown service: %s", service)))
		all := make([]string, 0)
		for k := range dockerSvcs {
			all = append(all, k)
		}
		for k := range winServiceNames {
			all = append(all, k)
		}
		fmt.Printf("  Available: %s\n", strings.Join(all, ", "))
	}
	fmt.Println()
}
