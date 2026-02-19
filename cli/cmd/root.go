package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "tunnel",
	Short: "Tunnel CLI - Expose local services to the internet",
	Long: `Tunnel is a CLI tool that allows you to expose local services to the internet
through secure tunnels, similar to ngrok or tunnel.to.

Examples:
  tunnel register                    # Register a new client
  tunnel start 3000                  # Start a tunnel on port 3000
  tunnel start 8080 --domain myapp   # Start a tunnel with custom domain
  tunnel list                        # List all active tunnels
  tunnel stop <tunnel-id>            # Stop a specific tunnel
  tunnel status                      # Show connection status`,
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Add global flags here if needed
}
