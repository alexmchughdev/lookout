package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/AlexMcHugh1/lookout/internal/config"
	"github.com/AlexMcHugh1/lookout/internal/report"
	"github.com/AlexMcHugh1/lookout/internal/runner"
	"github.com/AlexMcHugh1/lookout/internal/spec"
	"github.com/AlexMcHugh1/lookout/internal/vision"
)

var (
	flagURL      string
	flagEmail    string
	flagPassword string
	flagBuild    string
	flagSections string
	flagModel    string
	flagProvider string
	flagAPIKey   string
	flagOutput   string
	flagJUnit    string
	flagJSON     string
	flagRetries  int
	flagDebug    bool
	flagHeaded   bool
	flagNoReport bool
	flagNoPreflight bool
)

var runCmd = &cobra.Command{
	Use:   "run [SPEC]",
	Short: "Run a test suite against your app",
	Long: `Run a test suite against your app.

SPEC can be a YAML file or PDF document.
If omitted, uses lookout.yaml in the current directory.

Examples:
  lookout run tests.yaml
  lookout run tests.yaml --url https://staging.myapp.com
  lookout run spec.pdf   --url https://myapp.com --email me@co.com
  lookout run            --sections navigation,notes`,

	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		printHeader()
		return runSuite(args)
	},
}

func init() {
	runCmd.Flags().StringVarP(&flagURL, "url", "u", "", "App URL (overrides spec)")
	runCmd.Flags().StringVarP(&flagEmail, "email", "e", "", "Login email")
	runCmd.Flags().StringVarP(&flagPassword, "password", "p", "", "Login password")
	runCmd.Flags().StringVarP(&flagBuild, "build", "b", "", "Build ID for report")
	runCmd.Flags().StringVarP(&flagSections, "sections", "s", "", "Comma-separated sections to run")
	runCmd.Flags().StringVarP(&flagModel, "model", "m", "", "Vision model name")
	runCmd.Flags().StringVar(&flagProvider, "provider", "", "Model provider: ollama|anthropic|openai")
	runCmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for anthropic/openai")
	runCmd.Flags().StringVarP(&flagOutput, "output", "o", "reports", "Report output directory")
	runCmd.Flags().StringVar(&flagJUnit, "junit", "", "Write JUnit XML report to this path (for CI)")
	runCmd.Flags().StringVar(&flagJSON, "json", "", "Write machine-readable JSON report to this path")
	runCmd.Flags().IntVar(&flagRetries, "retry", 0, "Retry Fail/Blocked tests up to N times")
	runCmd.Flags().BoolVar(&flagDebug, "debug", false, "Embed all screenshots in report")
	runCmd.Flags().BoolVar(&flagHeaded, "headed", false, "Run browser in headed mode")
	runCmd.Flags().BoolVar(&flagNoReport, "no-report", false, "Skip HTML report generation")
	runCmd.Flags().BoolVar(&flagNoPreflight, "no-preflight", false, "Skip vision model reachability check")
}

func runSuite(args []string) error {
	// Resolve spec path
	specPath := "lookout.yaml"
	if len(args) > 0 {
		specPath = args[0]
	} else {
		for _, candidate := range []string{"lookout.yaml", "lookout.yml"} {
			if _, err := os.Stat(candidate); err == nil {
				specPath = candidate
				break
			}
		}
	}

	// Model override from flags
	var modelOverride *config.ModelConfig
	if flagProvider != "" || flagModel != "" || flagAPIKey != "" {
		m := config.ModelConfig{
			Provider: flagProvider,
			Name:     flagModel,
			APIKey:   flagAPIKey,
		}
		m.SetDefaults()
		modelOverride = &m
	}

	// Load spec
	dim := color.New(color.Faint)
	dim.Printf("  Loading spec: %s\n", specPath)

	s, err := spec.Load(specPath, modelOverride, flagURL)
	if err != nil {
		return fmt.Errorf("loading spec: %w", err)
	}
	if err := s.Validate(); err != nil {
		return err
	}

	// Apply CLI overrides
	if flagURL != "" {
		s.App.URL = strings.TrimRight(flagURL, "/")
	}
	if flagEmail != "" {
		s.App.Auth.Email = flagEmail
	}
	if flagPassword != "" {
		s.App.Auth.Password = flagPassword
	}
	if modelOverride != nil {
		s.Model = *modelOverride
	}

	// Credentials from env fallback
	if s.App.Auth.Email == "" {
		s.App.Auth.Email = os.Getenv("LOOKOUT_EMAIL")
	}
	if s.App.Auth.Password == "" {
		s.App.Auth.Password = os.Getenv("LOOKOUT_PASSWORD")
	}

	if s.App.Auth.Email == "" || s.App.Auth.Password == "" {
		return fmt.Errorf(
			"no credentials found\n" +
				"  Provide --email/--password, set them in the spec,\n" +
				"  or set LOOKOUT_EMAIL / LOOKOUT_PASSWORD env vars",
		)
	}

	buildID := flagBuild
	if buildID == "" {
		buildID = os.Getenv("LOOKOUT_BUILD")
	}
	if buildID == "" {
		buildID = "unknown"
	}

	var sections []string
	if flagSections != "" {
		for _, s := range strings.Split(flagSections, ",") {
			sections = append(sections, strings.TrimSpace(s))
		}
	}

	// Count tests to run
	tests := s.Tests
	if len(sections) > 0 {
		secSet := make(map[string]bool)
		for _, sec := range sections {
			secSet[sec] = true
		}
		filtered := tests[:0]
		for _, t := range tests {
			if secSet[t.Section] {
				filtered = append(filtered, t)
			}
		}
		tests = filtered
	}

	// Pre-run summary
	bold := color.New(color.Bold)
	fmt.Printf("  Target:   %s\n", s.App.URL)
	fmt.Printf("  Model:    %s/%s\n", s.Model.Provider, s.Model.Name)
	fmt.Printf("  Tests:    %d\n", len(tests))
	fmt.Printf("  Build:    %s\n", buildID)
	fmt.Println()

	// Preflight: fail fast if vision model is unreachable
	if !flagNoPreflight {
		if err := vision.Preflight(s.Model); err != nil {
			return fmt.Errorf("preflight failed: %w", err)
		}
	}

	sep := strings.Repeat("─", 52)
	fmt.Println(sep)
	bold.Println("  Running tests")
	fmt.Println(sep)

	// Colour helpers
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)
	faint := color.New(color.Faint)

	symbols := map[string]string{
		"Pass":    "✅",
		"Fail":    "❌",
		"Blocked": "⚠ ",
		"Skipped": "⏭ ",
	}

	t0 := time.Now()

	results, err := runner.Run(s, runner.Options{
		Sections: sections,
		Headless: !flagHeaded,
		Debug:    flagDebug,
		Retries:  flagRetries,
		OnResult: func(r *runner.Result) {
			sym := symbols[r.Verdict.Result]
			note := r.Verdict.Note
			if len(note) > 65 {
				note = note[:65]
			}
			prefix := fmt.Sprintf("  %s [%s] ", sym, r.TestID)
			suffix := fmt.Sprintf(" (%ds)", int(r.Duration.Seconds()))
			msg := note + suffix

			switch r.Verdict.Result {
			case "Pass":
				fmt.Printf("%s", prefix)
				green.Printf("Pass")
				faint.Printf(" — %s\n", msg)
			case "Fail":
				fmt.Printf("%s", prefix)
				red.Printf("Fail")
				faint.Printf(" — %s\n", msg)
			case "Blocked":
				fmt.Printf("%s", prefix)
				yellow.Printf("Blocked")
				faint.Printf(" — %s\n", msg)
			default:
				fmt.Printf("%s%s — %s\n", prefix, r.Verdict.Result, msg)
			}

			if r.PreActErr != "" {
				yellow.Printf("  ⚠  pre-action: %s\n", r.PreActErr)
			}
		},
	})

	duration := time.Since(t0)

	if err != nil {
		red.Printf("\n✗ %s\n", err)
		os.Exit(1)
	}

	// Summary
	passC, failC, blockC, skipC := 0, 0, 0, 0
	for _, r := range results {
		switch r.Verdict.Result {
		case "Pass":
			passC++
		case "Fail":
			failC++
		case "Blocked":
			blockC++
		case "Skipped":
			skipC++
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("═", 52))
	if failC == 0 {
		green.Printf("  ✅ %d passed", passC)
		faint.Printf("   %d skipped   %d blocked\n", skipC, blockC)
	} else {
		green.Printf("  ✅ %d passed", passC)
		fmt.Printf("   ")
		red.Printf("❌ %d failed", failC)
		faint.Printf("   %d skipped\n", skipC)
	}
	faint.Printf("  Done in %s\n", duration.Round(time.Second))

	// Report
	if !flagNoReport {
		path, err := report.Write(results, s, duration, flagOutput, buildID, flagDebug)
		if err != nil {
			yellow.Printf("  ⚠  report error: %v\n", err)
		} else {
			faint.Printf("  Report: %s\n", path)
		}
	}
	if flagJUnit != "" {
		if err := report.WriteJUnit(results, s, duration, flagJUnit); err != nil {
			yellow.Printf("  ⚠  junit error: %v\n", err)
		} else {
			faint.Printf("  JUnit:  %s\n", flagJUnit)
		}
	}
	if flagJSON != "" {
		if err := report.WriteJSON(results, s, duration, buildID, flagJSON); err != nil {
			yellow.Printf("  ⚠  json error: %v\n", err)
		} else {
			faint.Printf("  JSON:   %s\n", flagJSON)
		}
	}

	fmt.Println(strings.Repeat("═", 52))

	if failC > 0 {
		os.Exit(1)
	}
	return nil
}
