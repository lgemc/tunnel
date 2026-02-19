package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/lmanrique/tunnel/cli/internal/client"
	"github.com/lmanrique/tunnel/cli/internal/config"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tunnels",
	Long:  `List all tunnels associated with the current client, including both active and inactive tunnels.`,
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
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

	// List tunnels
	resp, err := apiClient.ListTunnels()
	if err != nil {
		return fmt.Errorf("failed to list tunnels: %w", err)
	}

	if resp.Count == 0 {
		fmt.Println("No tunnels found")
		return nil
	}

	// Print tunnels in a table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "TUNNEL ID\tDOMAIN\tSTATUS\tCREATED AT")
	fmt.Fprintln(w, "---------\t------\t------\t----------")

	for _, tunnel := range resp.Tunnels {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			tunnel.TunnelID,
			tunnel.Domain,
			tunnel.Status,
			tunnel.CreatedAt,
		)
	}

	w.Flush()

	fmt.Printf("\nTotal: %d tunnel(s)\n", resp.Count)

	return nil
}
