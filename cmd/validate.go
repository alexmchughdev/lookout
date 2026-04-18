package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/alexmchughdev/lookout/internal/config"
)

var validateCmd = &cobra.Command{
	Use:   "validate [SPEC]",
	Short: "Validate a lookout spec without running it",
	Long: `Parse a spec and check for missing required fields, duplicate IDs,
and malformed entries. Does not launch the browser or contact the vision model.

Returns exit code 0 on success, 1 on validation errors.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "lookout.yaml"
		if len(args) > 0 {
			path = args[0]
		} else {
			for _, candidate := range []string{"lookout.yaml", "lookout.yml"} {
				if _, err := os.Stat(candidate); err == nil {
					path = candidate
					break
				}
			}
		}

		spec, err := config.LoadYAML(path)
		if err != nil {
			return err
		}
		if err := spec.Validate(); err != nil {
			return err
		}

		green := color.New(color.FgGreen, color.Bold)
		faint := color.New(color.Faint)

		green.Printf("✓ %s is valid\n", path)
		faint.Printf("  URL:   %s\n", spec.App.URL)
		faint.Printf("  Model: %s/%s\n", spec.Model.Provider, spec.Model.Name)
		faint.Printf("  Tests: %d\n", len(spec.Tests))

		// Also surface a gentle warning if credentials are missing
		if spec.App.Auth.Email == "" && os.Getenv("LOOKOUT_EMAIL") == "" {
			yellow := color.New(color.FgYellow)
			yellow.Println("  ⚠  No login email set. Provide auth.email in the spec, --email, or LOOKOUT_EMAIL.")
		}
		if spec.App.Auth.Password == "" && os.Getenv("LOOKOUT_PASSWORD") == "" {
			yellow := color.New(color.FgYellow)
			yellow.Println("  ⚠  No login password set. Provide --password or LOOKOUT_PASSWORD.")
		}

		fmt.Println()
		return nil
	},
}
