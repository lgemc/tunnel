package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/lmanrique/tunnel/cli/internal/client"
	"github.com/lmanrique/tunnel/cli/internal/config"
	"github.com/lmanrique/tunnel/cli/internal/proxy"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start [port]",
	Short: "Start a tunnel to expose a local port",
	Long: `Start a tunnel to expose a local HTTP service to the internet.
The tunnel will forward all incoming requests to the specified local port.

Examples:
  tunnel start 3000                  # Start tunnel with random subdomain
  tunnel start 8080 --domain myapp   # Start tunnel with custom subdomain`,
	Args: cobra.ExactArgs(1),
	RunE: runStart,
}

var subdomain string

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringVar(&subdomain, "domain", "", "Custom subdomain (optional)")
}

func runStart(cmd *cobra.Command, args []string) error {
	// Parse port
	port, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid port: %w", err)
	}

	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

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

	if subdomain != "" {
		fmt.Printf("Connecting to tunnel for port %d (subdomain: %s)...\n", port, subdomain)
	} else {
		fmt.Printf("Creating tunnel for port %d...\n", port)
	}

	// Create tunnel
	tunnel, err := apiClient.CreateTunnel(subdomain)
	if err != nil {
		return fmt.Errorf("failed to create tunnel: %w", err)
	}

	if tunnel.Reused {
		fmt.Printf("\n✓ Reusing existing tunnel!\n")
	} else {
		fmt.Printf("\n✓ Tunnel created successfully!\n")
	}
	fmt.Printf("  Tunnel ID: %s\n", tunnel.TunnelID)
	fmt.Printf("  Domain:    %s\n", tunnel.Domain)
	fmt.Printf("  Status:    %s\n\n", tunnel.Status)
	fmt.Printf("Your local service is now accessible at: https://%s\n\n", tunnel.Domain)

	// Create and start proxy
	fmt.Println("Starting proxy...")

	proxyInstance := proxy.NewProxy(port, tunnel.WebsocketURL, cfg.APIKey, tunnel.TunnelID)

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Start proxy in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- proxyInstance.Start(ctx)
	}()

	fmt.Println("✓ Tunnel is now active!")
	fmt.Println("\nPress Ctrl+C to stop the tunnel")

	// Wait for interrupt or error
	select {
	case <-sigCh:
		fmt.Println("\n\nStopping tunnel...")
		cancel()
		// Wait for proxy to stop
		<-errCh
		fmt.Println("✓ Tunnel stopped")
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			return fmt.Errorf("proxy error: %w", err)
		}
	}

	return nil
}
