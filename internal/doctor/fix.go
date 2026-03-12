package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/ui"
)

type agentInfo struct {
	Cmd       string
	Name      string
	Flag      string
	ToolsFlag string
	Version   string
}

// Fix runs the interactive fix wizard.
func Fix(issues []Issue) {
	// Detect available AI CLI agents
	agents := detectAgents()
	if len(agents) == 0 {
		fmt.Printf("  %s No AI agent CLI found. Install one of:\n", ui.Err("✗"))
		fmt.Printf("    %s  npm install -g @anthropic-ai/claude-code\n", ui.Dim("Claude Code:"))
		fmt.Printf("    %s     go install github.com/opencode-ai/opencode@latest\n", ui.Dim("OpenCode:"))
		fmt.Printf("    %s        pip install aider-chat\n", ui.Dim("Aider:"))
		fmt.Printf("    %s        pip install goose-ai\n", ui.Dim("Goose:"))
		return
	}

	// Step 1: Select agent
	fmt.Printf("\n  %s\n", ui.Dim("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
	fmt.Printf("  %s %s — select your AI agent\n\n", ui.GoldText("⚓"), ui.Bold("Fix Wizard"))

	var agent agentInfo
	if len(agents) == 1 {
		agent = agents[0]
		fmt.Printf("  Using %s %s\n", ui.Bold(agent.Name), ui.Dim(agent.Version))
	} else {
		options := make([]huh.Option[int], len(agents))
		for i, a := range agents {
			options[i] = huh.NewOption(fmt.Sprintf("%s  %s", a.Name, ui.Dim(a.Version)), i)
		}
		var selected int
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[int]().
					Title("Select AI agent").
					Options(options...).
					Value(&selected),
			),
		)
		if err := form.Run(); err != nil {
			return
		}
		agent = agents[selected]
	}
	fmt.Printf("\n  %s Selected: %s\n\n", ui.Ok("✓"), ui.Bold(agent.Name))

	// Step 2: Build the prompt
	issueList := make([]string, len(issues))
	for i, iss := range issues {
		issueList[i] = fmt.Sprintf("  %d. %s", i+1, iss.Description)
	}

	host := config.Host()
	mediaWSL := config.MediaPathWSL()
	mediaWin := config.MediaPathWin()

	prompt := fmt.Sprintf(`You are the admirarr doctor fix wizard for a Plex + *Arr media server stack.

ENVIRONMENT:
- Windows host IP: %s
- Media path (WSL): %s  |  Media path (Windows): %s
- Docker containers: seerr, bazarr, organizr, flaresolverr
- Windows services: Sonarr, Radarr, Prowlarr, "Plex Media Server", Tautulli, qBittorrent

DETECTED ISSUES:
%s

FIX INSTRUCTIONS:
- Service unreachable (Docker): docker restart <name>, verify with curl
- Service unreachable (Windows): /mnt/c/Windows/System32/cmd.exe /c "powershell -Command \"Restart-Service '<Name>' -Force\""
- API key not found: read from /mnt/c/ProgramData/<Service>/config.xml, or guide user to Settings > General in the web UI
- Config missing: check alternative paths under /mnt/c/Users/*/AppData/
- Media path missing: mkdir -p <path>
- Disk space: report usage, suggest cleanup — DO NOT delete anything
- Docker container down: docker start <name>
- Indexer failures: check FlareSolverr (curl http://localhost:8191/v1), report Prowlarr status
- *Arr health warnings: explain and provide fix commands

For each issue print:
  Fixing: <description>
  Action: <command or step>
  Result: <outcome>

Fix what you can automatically. For manual steps, provide exact commands.`,
		host, mediaWSL, mediaWin, strings.Join(issueList, "\n"))

	// Step 3: Show prompt and confirm
	fmt.Printf("  %s\n", ui.Bold("Draft prompt:"))
	fmt.Printf("  %s\n", ui.Dim("──────────────────────────────────────────────────"))
	for _, line := range strings.Split(strings.TrimSpace(prompt), "\n") {
		fmt.Printf("  %s %s\n", ui.Dim("│"), line)
	}
	fmt.Printf("  %s\n\n", ui.Dim("──────────────────────────────────────────────────"))

	var action string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Run this fix?").
				Options(
					huh.NewOption("Yes — run the fix", "yes"),
					huh.NewOption("Edit prompt first", "edit"),
					huh.NewOption("Cancel", "no"),
				).
				Value(&action),
		),
	)
	if err := form.Run(); err != nil {
		return
	}

	if action == "no" {
		fmt.Printf("  %s\n", ui.Dim("Cancelled."))
		return
	}

	if action == "edit" {
		var edited string
		editForm := huh.NewForm(
			huh.NewGroup(
				huh.NewText().
					Title("Edit the prompt").
					Value(&edited).
					Lines(10),
			),
		)
		if err := editForm.Run(); err != nil {
			return
		}
		if edited != "" {
			prompt = edited
		}
		fmt.Printf("  %s Using custom prompt\n\n", ui.Ok("✓"))
	}

	// Step 4: Stream the fix
	fmt.Printf("  %s\n", ui.Dim("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
	fmt.Printf("  %s Streaming fixes via %s...\n\n", ui.GoldText("⚓"), ui.Bold(agent.Name))

	cmdParts := []string{agent.Cmd, agent.Flag, prompt}
	if agent.ToolsFlag != "" {
		cmdParts = append(cmdParts, strings.Fields(agent.ToolsFlag)...)
	}

	proc := exec.Command(cmdParts[0], cmdParts[1:]...)

	// Clear all CLAUDE* env vars so Claude Code doesn't detect a parent
	// session and refuse to run when admirarr is invoked from inside one.
	env := os.Environ()
	filteredEnv := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, "CLAUDECODE=") && !strings.HasPrefix(e, "CLAUDE_") {
			filteredEnv = append(filteredEnv, e)
		}
	}
	proc.Env = filteredEnv

	// Connect directly to terminal for real-time streaming (no pipe buffering)
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	proc.Stdin = os.Stdin

	if err := proc.Run(); err != nil {
		fmt.Printf("\n  %s\n", ui.Err(fmt.Sprintf("Agent exited with error: %v", err)))
	}

	fmt.Printf("\n  %s\n", ui.Dim("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
	fmt.Printf("  %s %s %s\n", ui.Dim("Re-run"), ui.GoldText("admirarr doctor"), ui.Dim("to verify fixes."))
}

func detectAgents() []agentInfo {
	agentDefs := []struct {
		Cmd       string
		Name      string
		Flag      string
		ToolsFlag string
	}{
		{"claude", "Claude Code", "-p", "--allowedTools Bash"},
		{"opencode", "OpenCode", "-p", ""},
		{"aider", "Aider", "--message", ""},
		{"goose", "Goose", "-m", ""},
	}

	var found []agentInfo
	for _, def := range agentDefs {
		out, err := exec.Command(def.Cmd, "--version").Output()
		if err != nil {
			continue
		}
		ver := strings.TrimSpace(strings.Split(string(out), "\n")[0])
		if len(ver) > 40 {
			ver = ver[:40]
		}
		found = append(found, agentInfo{
			Cmd:       def.Cmd,
			Name:      def.Name,
			Flag:      def.Flag,
			ToolsFlag: def.ToolsFlag,
			Version:   ver,
		})
	}
	return found
}
