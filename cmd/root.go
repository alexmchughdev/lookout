// Package cmd implements the lookout CLI.
package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

const asciiLogo = `
    __                __              __
   / /   ____  ____  / /______  __  _/ /_
  / /   / __ \/ __ \/ //_/ __ \/ / / / __/
 / /___/ /_/ / /_/ / ,< / /_/ / /_/ / /_
/_____/\____/\____/_/|_|\____/\__,_/\__/
`

const tagline = "visual QA · local-first · single binary"

var version = "0.1.0"

func printHeader() {
	cyan := color.New(color.FgCyan, color.Bold)
	dim := color.New(color.Faint)
	cyan.Println(asciiLogo)
	dim.Printf("  %s  v%s\n\n", tagline, version)
}

var rootCmd = &cobra.Command{
	Use:     "lookout",
	Short:   "Plug-and-play visual QA agent",
	Long:    asciiLogo + "\n  " + tagline,
	Version: version,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(modelsCmd)
	rootCmd.AddCommand(installBrowsersCmd)
}
