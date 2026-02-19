package cmd

import (
	"fmt"

	"github.com/lmanrique/tunnel/cli/internal/client"
	"github.com/lmanrique/tunnel/cli/internal/config"
	"github.com/spf13/cobra"
)

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register a new client with the tunnel service",
	Long: `Register a new client with the tunnel service and save credentials locally.
This command will create a new client ID and API key that will be used for all
subsequent tunnel operations.`,
	RunE: runRegister,
}

var (
	apiEndpoint string
	wsEndpoint  string
)

func init() {
	rootCmd.AddCommand(registerCmd)
	registerCmd.Flags().StringVar(&apiEndpoint, "api-endpoint", "", "API endpoint URL (required)")
	registerCmd.Flags().StringVar(&wsEndpoint, "ws-endpoint", "", "WebSocket endpoint URL (required)")
	registerCmd.MarkFlagRequired("api-endpoint")
	registerCmd.MarkFlagRequired("ws-endpoint")
}

func runRegister(cmd *cobra.Command, args []string) error {
	// Create API client
	apiClient := client.NewClient(apiEndpoint, "")

	fmt.Println("Registering new client...")

	// Register client
	resp, err := apiClient.RegisterClient()
	if err != nil {
		return fmt.Errorf("failed to register client: %w", err)
	}

	fmt.Printf("✓ Client registered successfully!\n")
	fmt.Printf("  Client ID: %s\n", resp.ClientID)
	fmt.Printf("  API Key:   %s\n\n", resp.APIKey)
	fmt.Println("⚠️  Please save your API key securely. It will not be shown again.")

	// Save config
	cfg := &config.Config{
		APIEndpoint:       apiEndpoint,
		WebSocketEndpoint: wsEndpoint,
		APIKey:            resp.APIKey,
		ClientID:          resp.ClientID,
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("\n✓ Configuration saved successfully!")
	fmt.Println("\nYou can now start using the tunnel service:")
	fmt.Println("  tunnel start 3000")

	return nil
}
