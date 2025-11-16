package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"

	"github.com/nbd-wtf/go-nostr/nip19"
	"gopkg.in/yaml.v3"
)

// Config holds application settings loaded from YAML config file
type Config struct {
	Relays     []string `yaml:"relays"`  // List of relay URLs to connect to
	Follows    []string `yaml:"follows"` // List of Nostr npubs to follow
	Quorum     int      `yaml:"quorum"`  // Number of follows needed to trigger action
	Network    string   `yaml:"network"` // Network identifier (e.g., "hqz", "testnet")
	NodeID     string   `yaml:"node_id"` // Unique node identifier
	ConfigPath string   `yaml:"-"`       // Path to config directory (not in YAML)
}

// generateNodeID creates a random UUID-like identifier for the node
func generateNodeID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		log.Printf("[WARN] Failed to generate random node ID: %v", err)
		return "node-unknown"
	}
	return fmt.Sprintf("node-%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// loadConfig reads the YAML config file or creates a default one if missing,
// then validates npubs and relay URLs.
func loadConfig(configDir string) Config {
	path := filepath.Join(configDir, "config.yaml")

	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Printf("[WARN] Config file not found at %s, creating from template", path)

		// Try to read from embedded template or fallback
		templatePath := "config.yaml.template"
		templateData, err := os.ReadFile(templatePath)
		if err != nil {
			// Fallback: create minimal config if template not found
			log.Printf("[WARN] Template file not found, creating minimal config")
			templateData = []byte(`# Qube Manager Configuration
relays:
  - wss://qubestr.zenon.info
  - wss://qubestr.zenon.red
follows:
  - npub1sr47j9awvw2xa0m4w770dr2rl7ylzq4xt9k5rel3h4h58sc3mjysx6pj64  # George
  - npub1ackp65pgrxp6r27jw82p68cv572r8yxgasnpaqnd2mzexr09gc3ss24gcw  # Vilkris
  - npub1mwwt7lxz5cyd3kgl5xmru8e2af2ajkuxrjsulyl6edwplwj36e3qkjwwaa  # Cryptofish
  - npub1aels8qtlje0m8q5z89pquk2cqq37kxzwzshafnmeccvlat3jqrpslh7rph  # Deeznnutz
  - npub1k52c552mgr75gzm8swar0y0nw4ctwwevlxtrx4ftvqypssafl3fsjgyt4v  # Coinselor
  - npub17uv2z8hrm90fuznz27xaxxagy7ysx5p9xfhqenq0yf3lueqnj8rqm70h8s  # Sl0th
quorum: 3
network: hqz
node_id: ""
`)
		}

		if err := os.WriteFile(path, templateData, 0644); err != nil {
			log.Fatalf("[ERROR] Failed to write default config to %s: %v", path, err)
		}
		log.Printf("[INFO] Default config created at %s", path)
		log.Printf("[INFO] Please review and edit %s before starting", path)
	} else if err != nil {
		log.Fatalf("[ERROR] Error checking config file %s: %v", path, err)
	} else {
		log.Printf("[INFO] Config file found at %s, loading", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("[ERROR] Failed to read config file %s: %v", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("[ERROR] Failed to parse config file %s: %v", path, err)
	}
	cfg.ConfigPath = configDir

	// Generate missing network/node_id for existing configs and save
	updated := false
	if cfg.Network == "" {
		log.Printf("[WARN] Config missing 'network' field, setting default: hqz")
		cfg.Network = "hqz"
		updated = true
	}
	if cfg.NodeID == "" {
		cfg.NodeID = generateNodeID()
		log.Printf("[WARN] Config missing 'node_id' field, generated: %s", cfg.NodeID)
		updated = true
	}
	if updated {
		data, err := yaml.Marshal(cfg)
		if err != nil {
			log.Printf("[WARN] Failed to marshal updated config: %v", err)
		} else if err := os.WriteFile(path, data, 0644); err != nil {
			log.Printf("[WARN] Failed to save updated config: %v", err)
		} else {
			log.Printf("[INFO] Updated config saved with network and node_id")
		}
	}

	log.Printf("[INFO] Loaded config: %d relay(s), %d follow(s), quorum=%d, network=%s, node_id=%s",
		len(cfg.Relays), len(cfg.Follows), cfg.Quorum, cfg.Network, cfg.NodeID)

	// Validate npubs
	for _, npub := range cfg.Follows {
		kind, _, err := nip19.Decode(npub)
		if err != nil {
			log.Fatalf("[ERROR] Invalid npub in config: %v", err)
		}
		if kind != "npub" {
			log.Fatalf("[ERROR] Expected npub but got %s in config: %s", kind, npub)
		}
	}

	// Validate relay URLs
	for _, r := range cfg.Relays {
		if _, err := url.ParseRequestURI(r); err != nil {
			log.Fatalf("[ERROR] Invalid relay URL in config: %s", r)
		}
	}

	return cfg
}
