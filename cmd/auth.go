package cmd

import (
	"bufio"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/alexmchughdev/lookout/internal/browser"
	"github.com/alexmchughdev/lookout/internal/config"
	"github.com/alexmchughdev/lookout/internal/spec"
)

var authCmd = &cobra.Command{
	Use:   "auth [SPEC]",
	Short: "Capture a login session (for apps behind MFA / SSO)",
	Long: `Open a headed browser so you can log in manually — including MFA,
SSO redirects, Microsoft / Okta / Google federation, whatever your
company uses. When you've finished logging in and can see the app,
press Enter in the terminal to save cookies + localStorage to the
session file.

Subsequent 'lookout run' with 'auth.type: session' in the spec will
reuse the session instead of attempting an automatic login. Re-run
this command whenever the session expires.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		printHeader()

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

		s, err := spec.Load(specPath, nil, "")
		if err != nil {
			return err
		}
		// We don't require a full Validate() here — the user might be running
		// `lookout auth` before they've finished authoring tests. We just need
		// app.url.
		if s.App.URL == "" {
			return fmt.Errorf("app.url is required in %s", specPath)
		}

		sessionFile := s.App.Auth.SessionFile
		if sessionFile == "" {
			sessionFile = config.DefaultSessionFile
		}

		faint := color.New(color.Faint)
		bold := color.New(color.Bold)
		green := color.New(color.FgGreen, color.Bold)

		faint.Printf("  Spec:    %s\n", specPath)
		faint.Printf("  Target:  %s\n", s.App.URL)
		faint.Printf("  Session: %s\n\n", sessionFile)

		session, err := browser.New(false) // headed
		if err != nil {
			return fmt.Errorf("launching browser: %w", err)
		}
		defer session.Cancel()

		if err := session.Navigate(s.App.URL); err != nil {
			return fmt.Errorf("navigating to %s: %w", s.App.URL, err)
		}

		bold.Println("  → Log in in the browser window (handle MFA, SSO, everything)")
		bold.Println("  → Wait until you see the app fully loaded")
		bold.Printf("  → Then press ")
		green.Print("Enter")
		bold.Println(" here to save the session")
		fmt.Println()

		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')

		// Make sure we're on the app origin so localStorage capture picks up
		// the right storage partition.
		if err := session.Navigate(s.App.URL); err != nil {
			return fmt.Errorf("returning to app origin before capture: %w", err)
		}

		if err := session.CaptureSession(s.App.URL, sessionFile); err != nil {
			return fmt.Errorf("saving session: %w", err)
		}

		green.Printf("✓ Session saved to %s\n", sessionFile)
		faint.Println("  Add it to .gitignore — it contains auth cookies.")
		fmt.Println()
		faint.Println("  In your spec set:")
		faint.Println("    auth:")
		faint.Println("      type: session")
		fmt.Println()
		faint.Println("  Then run: lookout run")
		return nil
	},
}
