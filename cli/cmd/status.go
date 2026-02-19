package cmd

import (
	"fmt"

	"github.com/lmanrique/tunnel/cli/internal/config"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show configuration status",
	Long:  `Display the current configuration status, including client ID, API endpoint, and connection details.`,
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !config.IsConfigured() {
		fmt.Println("Status: Not configured")
		fmt.Println("\nPlease run 'tunnel register' to get started")
		return nil
	}

	fmt.Println("Tunnel CLI Status")
	fmt.Println("=================")
	fmt.Printf("Status:        Configured\n")
	fmt.Printf("Client ID:     %s\n", cfg.ClientID)
	fmt.Printf("API Endpoint:  %s\n", cfg.APIEndpoint)
	fmt.Printf("WS Endpoint:   %s\n", cfg.WebSocketEndpoint)
	fmt.Printf("API Key:       %s...\n", maskAPIKey(cfg.APIKey))

	fmt.Println("\nConfiguration file location:")
	configDir, _ := config.GetConfigDir()
	fmt.Printf("  %s/config.yaml\n", configDir)

	return nil
}

func maskAPIKey(apiKey string) string {
	if len(apiKey) < 10 {
		return "****"
	}
	return apiKey[:10] + "****"
}
