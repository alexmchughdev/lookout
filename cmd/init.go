package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a starter lookout.yaml",
	RunE: func(cmd *cobra.Command, args []string) error {
		printHeader()

		urlFlag, _ := cmd.Flags().GetString("url")
		emailFlag, _ := cmd.Flags().GetString("email")

		if urlFlag == "" {
			fmt.Print("App URL: ")
			fmt.Scan(&urlFlag)
		}
		if emailFlag == "" {
			fmt.Print("Login email: ")
			fmt.Scan(&emailFlag)
		}

		template := fmt.Sprintf(`# lookout test spec
# Run: lookout run

app:
  url: %s
  auth:
    type: email_password
    email: %s
    password: ""   # or: export LOOKOUT_PASSWORD='...'
    # Uncomment for two-step login (email → Continue → password):
    # continue_button: 'button:has-text("Continue")'

# Model config — defaults to local Ollama (free, no API key needed)
# model:
#   provider: ollama
#   name: gemma3:12b
#   host: http://localhost:11434
#
# To use Claude API instead:
#   provider: anthropic
#   name: claude-sonnet-4-5
#   api_key: sk-ant-...   # or: export LOOKOUT_API_KEY='...'

tests:
  - id: smoke-01
    section: smoke
    url: /
    question: Does the app load without a blank white screen or error message?

  - id: auth-01
    section: auth
    url: /login
    question: Is a login form visible with email and password fields?

  # Add more tests here. Each test navigates to `+"`url`"+`, optionally runs
  # a pre_action, then asks `+"`question`"+` about a screenshot.
  #
  # Available pre_action types:
  #   click         — click an element
  #   type_and_verify — type text, save, reload, verify text persists
  #   open_first    — click first item in a list (e.g. open a workflow)
  #   drag          — drag a card (React DnD compatible)
  #   new_item      — click a New/Create button
  #   select_option — click first option in a list
  #
  # Example:
  # - id: notes-01
  #   section: notes
  #   url: /notes
  #   question: Do folders and notes load without an infinite spinner?
  #   pre_action:
  #     type: click
  #     selector: 'text=My Note'
`, urlFlag, emailFlag)

		out := "lookout.yaml"
		if _, err := os.Stat(out); err == nil {
			fmt.Print("lookout.yaml already exists. Overwrite? [y/N]: ")
			var answer string
			fmt.Scan(&answer)
			if answer != "y" && answer != "Y" {
				return nil
			}
		}

		if err := os.WriteFile(out, []byte(template), 0644); err != nil {
			return fmt.Errorf("writing lookout.yaml: %w", err)
		}

		fmt.Println("✓ Created lookout.yaml")
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  1. Add your tests to lookout.yaml")
		fmt.Println("  2. export LOOKOUT_PASSWORD='yourpassword'")
		fmt.Println("  3. lookout run")

		return nil
	},
}

func init() {
	initCmd.Flags().StringP("url", "u", "", "App URL")
	initCmd.Flags().StringP("email", "e", "", "Login email")
}

// ── lookout models ────────────────────────────────────────────────────────────

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List recommended vision models",
	Run: func(cmd *cobra.Command, args []string) {
		printHeader()
		fmt.Println("Recommended vision models:")
		fmt.Println()
		fmt.Printf("  %-28s %-12s %-8s %s\n", "Model", "Provider", "VRAM", "Notes")
		fmt.Printf("  %-28s %-12s %-8s %s\n",
			"----------------------------", "------------", "--------", "-----")

		rows := [][]string{
			{"gemma3:12b", "ollama", "~8GB", "Best local default. Vision capable."},
			{"qwen2.5vl:7b", "ollama", "~5GB", "Faster, less VRAM."},
			{"llama3.2-vision:11b", "ollama", "~7GB", "Strong vision."},
			{"claude-sonnet-4-5", "anthropic", "API", "Highest accuracy. Requires API key."},
			{"gpt-4o", "openai", "API", "Strong vision. Requires API key."},
		}

		for _, r := range rows {
			fmt.Printf("  %-28s %-12s %-8s %s\n", r[0], r[1], r[2], r[3])
		}

		fmt.Println()
		fmt.Println("Pull a local model:  ollama pull gemma3:12b")
		fmt.Println("Use API model:       add api_key to lookout.yaml")
	},
}

// ── lookout install-browsers ──────────────────────────────────────────────────

var installBrowsersCmd = &cobra.Command{
	Use:   "install-browsers",
	Short: "Install Chromium for lookout to drive",
	Run: func(cmd *cobra.Command, args []string) {
		printHeader()
		fmt.Println("lookout uses chromedp which drives system Chromium.")
		fmt.Println()
		fmt.Println("Install Chromium:")
		fmt.Println("  Ubuntu/Debian:  sudo apt install chromium-browser")
		fmt.Println("  Arch:           sudo pacman -S chromium")
		fmt.Println("  macOS:          brew install --cask chromium")
		fmt.Println()
		fmt.Println("Alternatively, install Playwright's bundled browser:")
		fmt.Println("  pip install playwright && playwright install chromium")
		fmt.Println("  (lookout will find it automatically at ~/.cache/ms-playwright/)")
	},
}
