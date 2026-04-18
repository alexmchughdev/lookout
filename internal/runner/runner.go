// Package runner orchestrates the full test run.
package runner

import (
	"fmt"
	"strings"
	"time"

	"github.com/AlexMcHugh1/lookout/internal/auth"
	"github.com/AlexMcHugh1/lookout/internal/browser"
	"github.com/AlexMcHugh1/lookout/internal/config"
	"github.com/AlexMcHugh1/lookout/internal/preactions"
	"github.com/AlexMcHugh1/lookout/internal/vision"
)

// Result holds the outcome of a single test.
type Result struct {
	TestID     string
	Section    string
	Verdict    vision.Verdict
	Duration   time.Duration
	Attempts   int    // how many attempts were made (>=1)
	Screenshot []byte // always populated; the report decides whether to embed
	PreActErr  string // non-empty if pre-action failed (test still runs)
}

// Options configures a test run.
type Options struct {
	Sections []string         // nil = all sections
	Headless bool             // true = headless Chrome
	Retries  int              // retry Fail/Blocked verdicts up to N extra times
	OnResult func(r *Result)  // called after each test completes
}

// Run executes all tests in the spec and returns results.
func Run(spec *config.Spec, opts Options) ([]*Result, error) {
	// Filter tests by section
	tests := spec.Tests
	if len(opts.Sections) > 0 {
		sectionSet := make(map[string]bool)
		for _, s := range opts.Sections {
			sectionSet[strings.TrimSpace(s)] = true
		}
		filtered := tests[:0]
		for _, t := range tests {
			if sectionSet[t.Section] {
				filtered = append(filtered, t)
			}
		}
		tests = filtered
	}

	if len(tests) == 0 {
		return nil, fmt.Errorf("no tests to run (check --sections filter)")
	}

	// Launch browser
	session, err := browser.New(opts.Headless)
	if err != nil {
		return nil, fmt.Errorf("launching browser: %w\nTip: run 'lookout install-browsers'", err)
	}
	defer session.Cancel()

	// Authenticate
	if spec.App.Auth.Type == "session" {
		// Navigate to app origin first so localStorage writes land in the right partition.
		if err := session.Navigate(spec.App.URL); err != nil {
			return nil, fmt.Errorf("navigating to app before session restore: %w", err)
		}
		if _, err := session.RestoreSession(spec.App.Auth.SessionFile); err != nil {
			return nil, err
		}
		// Reload so the server sees the injected cookies.
		if err := session.Navigate(spec.App.URL); err != nil {
			return nil, fmt.Errorf("navigating after session restore: %w", err)
		}
		// If we land on the login page, the session has expired.
		if exclude := spec.App.Auth.SuccessURLExcludes; exclude != "" {
			u, _ := session.CurrentURL()
			if strings.Contains(u, exclude) {
				return nil, fmt.Errorf(
					"session appears expired (landed on %s) — re-run 'lookout auth'",
					u,
				)
			}
		}
	} else {
		if err := auth.Login(session, spec.App.URL, spec.App.Auth); err != nil {
			return nil, err
		}
	}

	// Run tests
	var results []*Result
	for i := range tests {
		var r *Result
		attempts := opts.Retries + 1
		for attempt := 1; attempt <= attempts; attempt++ {
			r = runOne(session, &tests[i], spec)
			r.Attempts = attempt
			if r.Verdict.Result == "Pass" || r.Verdict.Result == "Skipped" {
				break
			}
			if attempt < attempts {
				time.Sleep(1 * time.Second)
			}
		}
		results = append(results, r)
		if opts.OnResult != nil {
			opts.OnResult(r)
		}
	}

	return results, nil
}

func runOne(s *browser.Session, test *config.TestDef, spec *config.Spec) *Result {
	r := &Result{
		TestID:  test.ID,
		Section: test.Section,
	}

	// Navigate
	url := strings.TrimRight(spec.App.URL, "/") + test.URL
	if err := s.Navigate(url); err != nil {
		r.Verdict = vision.Verdict{
			Result: "Blocked",
			Note:   fmt.Sprintf("navigation failed: %v", err),
		}
		return r
	}

	// Pre-action
	if test.PreAction != nil {
		if err := preactions.Run(s, spec.App.URL, test.PreAction); err != nil {
			r.PreActErr = err.Error()
			// Don't abort — still take the screenshot and assess
		}
	}

	// Optional wait_for selector (e.g. SPA readiness signal)
	if test.WaitFor != "" {
		_ = s.WaitForSelector(test.WaitFor, 15*time.Second)
	}

	// Optional extra settle time
	if test.WaitMs > 0 {
		s.Sleep(time.Duration(test.WaitMs) * time.Millisecond)
	}

	// Screenshot — default to full-page unless explicitly disabled
	start := time.Now()
	fullPage := true
	if test.FullPage != nil {
		fullPage = *test.FullPage
	}
	var screenshot []byte
	var err error
	if fullPage {
		screenshot, err = s.FullPageScreenshot()
	} else {
		screenshot, err = s.Screenshot()
	}
	if err != nil {
		r.Verdict = vision.Verdict{
			Result: "Blocked",
			Note:   fmt.Sprintf("screenshot failed: %v", err),
		}
		return r
	}

	// Judge
	verdict, err := vision.Judge(screenshot, test.Question, spec.Model)
	if err != nil {
		r.Verdict = vision.Verdict{Result: "Blocked", Note: err.Error()}
	} else {
		r.Verdict = verdict
	}

	r.Duration = time.Since(start)

	// Always retain the screenshot — the report decides whether to embed it.
	// Memory cost is tiny (~200-500 KB per test) and having it available means
	// users never wonder "what did the model actually see?".
	r.Screenshot = screenshot

	return r
}
