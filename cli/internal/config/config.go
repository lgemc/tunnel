package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	ConfigDir  = ".tunnel"
	ConfigFile = "config"
)

// Config represents the CLI configuration
type Config struct {
	APIEndpoint       string `mapstructure:"api_endpoint"`
	WebSocketEndpoint string `mapstructure:"websocket_endpoint"`
	APIKey            string `mapstructure:"api_key"`
	ClientID          string `mapstructure:"client_id"`
}

// GetConfigDir returns the configuration directory path
func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(home, ConfigDir), nil
}

// EnsureConfigDir ensures the configuration directory exists
func EnsureConfigDir() error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return nil
}

// Load loads the configuration from file
func Load() (*Config, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}

	viper.SetConfigName(ConfigFile)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configDir)

	// Set defaults
	viper.SetDefault("api_endpoint", "")
	viper.SetDefault("websocket_endpoint", "")
	viper.SetDefault("api_key", "")
	viper.SetDefault("client_id", "")

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found, return empty config
			return &Config{}, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// Save saves the configuration to file
func Save(config *Config) error {
	if err := EnsureConfigDir(); err != nil {
		return err
	}

	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	viper.Set("api_endpoint", config.APIEndpoint)
	viper.Set("websocket_endpoint", config.WebSocketEndpoint)
	viper.Set("api_key", config.APIKey)
	viper.Set("client_id", config.ClientID)

	configPath := filepath.Join(configDir, ConfigFile+".yaml")
	if err := viper.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// Clear removes the configuration file
func Clear() error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	configPath := filepath.Join(configDir, ConfigFile+".yaml")
	if err := os.Remove(configPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove config: %w", err)
		}
	}

	return nil
}

// IsConfigured checks if the CLI is configured
func IsConfigured() bool {
	config, err := Load()
	if err != nil {
		return false
	}

	return config.APIKey != "" && config.ClientID != "" && config.APIEndpoint != ""
}
