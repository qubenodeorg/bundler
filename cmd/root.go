package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "qubenode-bundler",
	Short: "ERC-4337 Bundler",
	Long:  "A modular Go implementation of an ERC-4337 Bundler.",
}

// SetVersion sets the build version reported by `--version`. It is wired from
// main with a value injected at build time via -ldflags "-X main.version=...".
func SetVersion(v string) {
	rootCmd.Version = v
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {}
