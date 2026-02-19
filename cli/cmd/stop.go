package cmd

import (
	"fmt"

	"github.com/lmanrique/tunnel/cli/internal/client"
	"github.com/lmanrique/tunnel/cli/internal/config"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop [tunnel-id]",
	Short: "Stop and delete a tunnel",
	Long: `Stop and delete a tunnel by its ID.
This will permanently remove the tunnel and its associated domain.

Example:
  tunnel stop abc123def456`,
	Args: cobra.ExactArgs(1),
	RunE: runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	tunnelID := args[0]

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !config.IsConfigured() {
		return fmt.Errorf("not configured. Please run 'tunnel register' first")
	}

	// Create API client
	apiClient := client.NewClient(cfg.APIEndpoint, cfg.APIKey)

	fmt.Printf("Stopping tunnel %s...\n", tunnelID)

	// Delete tunnel
	if err := apiClient.DeleteTunnel(tunnelID); err != nil {
		return fmt.Errorf("failed to stop tunnel: %w", err)
	}

	fmt.Println("✓ Tunnel stopped successfully!")

	return nil
}
