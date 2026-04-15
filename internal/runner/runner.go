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
	Screenshot []byte // nil unless debug mode or failure
	PreActErr  string // non-empty if pre-action failed (test still runs)
}

// Options configures a test run.
type Options struct {
	Sections []string          // nil = all sections
	Headless bool              // true = headless Chrome
	Debug    bool              // true = embed all screenshots
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

	// Login
	if err := auth.Login(session, spec.App.URL, spec.App.Auth); err != nil {
		return nil, err
	}

	// Run tests
	var results []*Result
	for i := range tests {
		r := runOne(session, &tests[i], spec, opts.Debug)
		results = append(results, r)
		if opts.OnResult != nil {
			opts.OnResult(r)
		}
	}

	return results, nil
}

func runOne(s *browser.Session, test *config.TestDef, spec *config.Spec, debug bool) *Result {
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

	// Screenshot
	start      := time.Now()
	screenshot, err := s.Screenshot()
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

	// Store screenshot if debug or failure
	if debug || r.Verdict.Result == "Fail" {
		r.Screenshot = screenshot
	}

	return r
}
