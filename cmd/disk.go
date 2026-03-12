package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/ui"
	"github.com/spf13/cobra"
)

var diskCmd = &cobra.Command{
	Use:   "disk",
	Short: "Disk space breakdown",
	Long: `Show disk space breakdown for media directories.

Shows disk usage for the media drive and subdirectories:
  D:\Media, Downloads/, Movies/, TV Shows/`,
	Example: "  admirarr disk",
	Run:     runDisk,
}

func init() {
	rootCmd.AddCommand(diskCmd)
}

func runDisk(cmd *cobra.Command, args []string) {
	ui.PrintBanner()
	fmt.Println(ui.Bold("\n  Disk Space\n"))

	mediaWSL := config.MediaPathWSL()
	paths := []struct {
		path  string
		label string
	}{
		{mediaWSL, `D:\Media`},
		{filepath.Join(mediaWSL, "Downloads"), "Downloads"},
		{filepath.Join(mediaWSL, "Movies"), "Movies"},
		{filepath.Join(mediaWSL, "TV Shows"), "TV Shows"},
	}

	for _, p := range paths {
		total, free, err := getStatfs(p.path)
		if err != nil {
			fmt.Printf("  %-15s %s\n", p.label, ui.Err("N/A"))
			continue
		}

		used := total - free

		// Count files
		count := 0
		_ = filepath.Walk(p.path, func(_ string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() {
				count++
			}
			return nil
		})

		fmt.Printf("  %-15s %10s used / %10s total  (%d files)\n",
			p.label, ui.FmtSize(used), ui.FmtSize(total), count)
	}
	fmt.Println()
}
