package cmd

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

const defaultHealthURL = "http://127.0.0.1:4337/live"

var healthcheckCmd = &cobra.Command{
	Use:   "healthcheck",
	Short: "Performs an HTTP GET against the liveness endpoint",
	Long:  `Exits 0 on HTTP 200, non-zero otherwise. Used as a Docker HEALTHCHECK in distroless images.`,
	Run: func(cmd *cobra.Command, args []string) {
		url := defaultHealthURL
		if len(args) > 0 {
			url = args[0]
		}

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "healthcheck failed: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			_, _ = fmt.Fprintf(os.Stderr, "healthcheck failed: status %d\n", resp.StatusCode)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(healthcheckCmd)
}
