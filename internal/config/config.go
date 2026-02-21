package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Daemon DaemonConfig `yaml:"daemon"`
	Client ClientConfig `yaml:"client"`
}

type DaemonConfig struct {
	DBPath  string `yaml:"db_path"`
	CASDir  string `yaml:"cas_dir"`
	APIPort int    `yaml:"api_port"`
	P2PPort int    `yaml:"p2p_port"`
	Profile string `yaml:"profile"`
}

type ClientConfig struct {
	APIUrl string `yaml:"api_url"`
}

const DefaultConfigTemplate = `# Resilient Knowledge Vault Configuration
# 
# This file governs both the background 'vaultd' daemon and the 'vault' CLI client.
# All options below are set to their defaults but are commented out.
# Uncomment and change specific lines to override the built-in defaults.

daemon:
  # Path to the local SQLite database file. Uses WAL mode.
  # db_path: "vault.db"

  # Path to the immutable CAS (Content Addressable Storage) directory.
  # cas_dir: "vault_cas"

  # Port to expose the local HTTP API and Web UI (Localhost only natively).
  # api_port: 8080

  # Port to bind the Libp2p network mesh listener (TCP, UDP QUIC, UDP WebRTC).
  # p2p_port: 4001

  # Network profile strategy: 'standard', 'hub', or 'stealth'.
  # profile: "standard"

client:
  # The endpoint URL that the CLI and TUI use to interact with the local daemon.
  # api_url: "http://127.0.0.1:8080"
`

// DefaultConfig returns an initialized config with fallback default values.
func DefaultConfig() *Config {
	return &Config{
		Daemon: DaemonConfig{
			DBPath:  "vault.db",
			CASDir:  "vault_cas",
			APIPort: 8080,
			P2PPort: 4001,
			Profile: "standard",
		},
		Client: ClientConfig{
			APIUrl: "http://127.0.0.1:8080",
		},
	}
}

// GenerateTemplate writes the heavily annotated configuration template to the specified path.
func GenerateTemplate(path string) error {
	return os.WriteFile(path, []byte(DefaultConfigTemplate), 0644)
}

// Load reads the YAML configuration file from the specified path.
// It initializes with default values first, then overwrites them with any values found in the YAML.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Fail cleanly if the user specifically requested a file that doesn't exist
			return nil, fmt.Errorf("config file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse yaml config: %w", err)
	}

	return cfg, nil
}
