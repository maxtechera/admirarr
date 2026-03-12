package setup

import (
	"fmt"

	"github.com/maxtechera/admirarr/internal/ui"
)

// SetupState accumulates validated configuration across all phases.
type SetupState struct {
	Host     string
	MediaWSL string
	MediaWin string
	Services map[string]*ServiceState
	Keys     map[string]string
	// ManualKeys tracks keys the user entered manually (need persisting).
	ManualKeys map[string]string
}

// ServiceState tracks the detected state of a single service.
type ServiceState struct {
	Detected  bool
	Reachable bool
	Host      string
	Port      int
	Type      string // "windows" or "docker"
	APIKey    string
}

// StepResult tracks what happened in each phase.
type StepResult struct {
	Name    string
	Passed  int
	Fixed   int
	Skipped int
	Errors  []string
}

func (r *StepResult) pass()                { r.Passed++ }
func (r *StepResult) fix()                 { r.Fixed++ }
func (r *StepResult) skip()                { r.Skipped++ }
func (r *StepResult) err(msg string)       { r.Errors = append(r.Errors, msg) }
func (r *StepResult) errf(f string, a ...interface{}) {
	r.Errors = append(r.Errors, fmt.Sprintf(f, a...))
}

// Run executes the full setup wizard.
func Run() {
	state := &SetupState{
		Services:   make(map[string]*ServiceState),
		Keys:       make(map[string]string),
		ManualKeys: make(map[string]string),
	}

	var results []StepResult

	// Phase 1: Environment Detection
	r := Detect(state)
	results = append(results, r)
	printStepSummary(r)

	// Phase 2: Service Connectivity
	r = CheckConnectivity(state)
	results = append(results, r)
	printStepSummary(r)

	// Phase 3: API Key Validation
	r = ValidateAPIKeys(state)
	results = append(results, r)
	printStepSummary(r)

	// Phase 4: Download Client Configuration
	r = ConfigureDownloadClients(state)
	results = append(results, r)
	printStepSummary(r)

	// Phase 5: Root Folders & Media Paths
	r = ValidateRootFolders(state)
	results = append(results, r)
	printStepSummary(r)

	// Phase 6: Indexer Sync
	r = VerifyIndexers(state)
	results = append(results, r)
	printStepSummary(r)

	// Phase 7: Quality Profile Sync
	r = SyncQualityProfiles(state)
	results = append(results, r)
	printStepSummary(r)

	// Phase 8: Write Config
	r = WriteConfig(state)
	results = append(results, r)
	printStepSummary(r)

	// Final summary
	printFinalSummary(results)
}

func printStepSummary(r StepResult) {
	parts := []string{}
	if r.Passed > 0 {
		parts = append(parts, ui.Ok(fmt.Sprintf("%d passed", r.Passed)))
	}
	if r.Fixed > 0 {
		parts = append(parts, ui.GoldText(fmt.Sprintf("%d fixed", r.Fixed)))
	}
	if r.Skipped > 0 {
		parts = append(parts, ui.Dim(fmt.Sprintf("%d skipped", r.Skipped)))
	}
	if len(r.Errors) > 0 {
		parts = append(parts, ui.Err(fmt.Sprintf("%d error(s)", len(r.Errors))))
	}
	summary := ""
	for i, p := range parts {
		if i > 0 {
			summary += ", "
		}
		summary += p
	}
	fmt.Printf("\n  %s %s: %s\n", ui.Dim("→"), ui.Bold(r.Name), summary)
}

func printFinalSummary(results []StepResult) {
	fmt.Println(ui.Separator())
	totalPassed, totalFixed, totalErrors := 0, 0, 0
	for _, r := range results {
		totalPassed += r.Passed
		totalFixed += r.Fixed
		totalErrors += len(r.Errors)
	}

	if totalErrors == 0 {
		fmt.Printf("\n  %s Setup complete! %s passed, %s fixed\n",
			ui.Ok("✓"), ui.Ok(fmt.Sprintf("%d", totalPassed)), ui.GoldText(fmt.Sprintf("%d", totalFixed)))
	} else {
		fmt.Printf("\n  %s Setup finished with issues: %s passed, %s fixed, %s\n",
			ui.Warn("!"), ui.Ok(fmt.Sprintf("%d", totalPassed)),
			ui.GoldText(fmt.Sprintf("%d", totalFixed)),
			ui.Err(fmt.Sprintf("%d error(s)", totalErrors)))
		fmt.Println()
		for _, r := range results {
			for _, e := range r.Errors {
				fmt.Printf("  %s %s\n", ui.Err("✗"), e)
			}
		}
	}

	fmt.Printf("\n  %s Run %s to verify.\n\n", ui.GoldText("⚓"), ui.GoldText("admirarr doctor"))
}
