// Package auth handles deterministic login via the browser session.
package auth

import (
	"fmt"
	"strings"
	"time"

	"github.com/alexmchughdev/lookout/internal/browser"
	"github.com/alexmchughdev/lookout/internal/config"
)

// Login logs into the app using the provided auth config.
// Handles both single-step and two-step (email → Continue → password) flows.
func Login(s *browser.Session, baseURL string, auth config.AuthConfig) error {
	path := auth.LoginPath
	if path == "" {
		path = "/login"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	loginURL := strings.TrimRight(baseURL, "/") + path

	if err := s.Navigate(loginURL); err != nil {
		return fmt.Errorf("navigating to login page: %w", err)
	}

	// Fill email
	if err := s.Fill(auth.EmailField, auth.Email); err != nil {
		return fmt.Errorf("filling email field (%s): %w", auth.EmailField, err)
	}

	// Two-step flow: click Continue button if configured
	if auth.ContinueButton != "" {
		if err := s.Click(auth.ContinueButton); err != nil {
			return fmt.Errorf("clicking continue button: %w", err)
		}
		s.Sleep(1 * time.Second)
	} else {
		// Auto-detect: if password field is not immediately visible, try clicking a Next button
		s.Sleep(500 * time.Millisecond)
		s.ClickIfExists(`button:has-text("Continue"), button:has-text("Next")`)
		s.Sleep(500 * time.Millisecond)
	}

	// Fill password
	if err := s.Fill(auth.PasswordField, auth.Password); err != nil {
		return fmt.Errorf("filling password field: %w", err)
	}

	// Submit
	if err := s.Click(auth.SubmitButton); err != nil {
		// Try common fallbacks
		s.ClickIfExists(`button:has-text("Sign in")`)
		s.ClickIfExists(`button:has-text("Log in")`)
	}

	// Wait for redirect away from login page
	if err := s.WaitForURLExcludes(auth.SuccessURLExcludes, 15*time.Second); err != nil {
		return fmt.Errorf("login failed — still on %s page after 15s.\n"+
			"Check credentials and auth config in your spec file", auth.SuccessURLExcludes)
	}

	// Let the SPA hydrate
	s.Sleep(3 * time.Second)

	u, _ := s.CurrentURL()
	_ = u
	return nil
}
