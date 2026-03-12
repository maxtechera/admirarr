package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/maxtechera/admirarr/internal/ui"
	"github.com/spf13/cobra"
)

var dockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Docker container status",
	Long: `Show Docker container status for media stack containers.

Filters for: seerr, bazarr, organizr, flaresolverr.

Uses: docker ps -a`,
	Example: "  admirarr docker",
	Run:     runDocker,
}

func init() {
	rootCmd.AddCommand(dockerCmd)
}

func runDocker(cmd *cobra.Command, args []string) {
	ui.PrintBanner()
	fmt.Println(ui.Bold("\n  Docker Containers\n"))

	out, err := exec.Command("docker", "ps", "-a", "--format", "{{.Names}}\t{{.Status}}\t{{.Ports}}",
		"--filter", "name=seerr", "--filter", "name=bazarr",
		"--filter", "name=organizr", "--filter", "name=flaresolverr").Output()
	if err != nil {
		fmt.Printf("  %s\n", ui.Dim("No containers found"))
		fmt.Println()
		return
	}

	lines := strings.TrimSpace(string(out))
	if lines == "" {
		fmt.Printf("  %s\n", ui.Dim("No containers found"))
		fmt.Println()
		return
	}

	for _, line := range strings.Split(lines, "\n") {
		parts := strings.SplitN(line, "\t", 3)
		name := "?"
		status := "?"
		if len(parts) > 0 {
			name = parts[0]
		}
		if len(parts) > 1 {
			status = parts[1]
		}
		colorFn := ui.Err
		if strings.Contains(status, "Up") {
			colorFn = ui.Ok
		}
		fmt.Printf("  %-15s %s\n", name, colorFn(status))
	}
	fmt.Println()
}
