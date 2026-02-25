package cmd

import (
	"fmt"
	"time"

	"github.com/lmanrique/tunnel/cli/internal/client"
	"github.com/lmanrique/tunnel/cli/internal/config"
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test [tunnel-id]",
	Short: "Test if a tunnel is working",
	Long: `Test if a tunnel is working by making a health check request to its public URL.
If no tunnel ID is provided, tests all active tunnels.

Examples:
  tunnel test abc123           # Test specific tunnel
  tunnel test                  # Test all tunnels`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTest,
}

func init() {
	rootCmd.AddCommand(testCmd)
}

func runTest(cmd *cobra.Command, args []string) error {
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

	// Get list of tunnels
	resp, err := apiClient.ListTunnels()
	if err != nil {
		return fmt.Errorf("failed to list tunnels: %w", err)
	}

	if len(resp.Tunnels) == 0 {
		fmt.Println("No tunnels found")
		return nil
	}

	// Filter tunnels if specific ID provided
	var tunnelsToTest []client.Tunnel
	if len(args) == 1 {
		targetID := args[0]
		found := false
		for _, tunnel := range resp.Tunnels {
			if tunnel.TunnelID == targetID {
				tunnelsToTest = append(tunnelsToTest, tunnel)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("tunnel %s not found", targetID)
		}
	} else {
		// Test all active tunnels
		for _, tunnel := range resp.Tunnels {
			if tunnel.Status == "active" {
				tunnelsToTest = append(tunnelsToTest, tunnel)
			}
		}
	}

	if len(tunnelsToTest) == 0 {
		fmt.Println("No active tunnels to test")
		return nil
	}

	// Test each tunnel
	fmt.Printf("Testing %d tunnel(s)...\n\n", len(tunnelsToTest))
	successCount := 0
	failCount := 0

	for _, tunnel := range tunnelsToTest {
		fmt.Printf("Testing %s (https://%s)... ", tunnel.TunnelID, tunnel.Domain)

		start := time.Now()
		err := apiClient.TestTunnel(tunnel.Domain)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("✗ FAILED (%v)\n", duration)
			fmt.Printf("  Error: %v\n\n", err)
			failCount++
		} else {
			fmt.Printf("✓ OK (%v)\n\n", duration)
			successCount++
		}
	}

	// Summary
	fmt.Printf("Summary: %d passed, %d failed\n", successCount, failCount)

	if failCount > 0 {
		return fmt.Errorf("%d tunnel(s) failed health check", failCount)
	}

	return nil
}
