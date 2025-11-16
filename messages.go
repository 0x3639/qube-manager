package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

// UpgradeMessage represents the "upgrade" message type
type UpgradeMessage struct {
	Type      string `json:"type"`                // Must be "upgrade"
	Version   string `json:"version"`             // Semantic version string
	ExtraData string `json:"extraData,omitempty"` // additional metadata or status
}

// RebootMessage represents the "reboot" message type
type RebootMessage struct {
	Type      string `json:"type"`                // Must be "reboot"
	Version   string `json:"version"`             // Semantic version string
	Genesis   string `json:"genesis"`             // URL string
	ExtraData string `json:"extraData,omitempty"` // additional metadata or status
}

func sendMessageCLI(configDir string) {
	var (
		msgType    string
		version    string
		genesis    string
		hash       string
		network    string
		requiredBy string
		dryRun     bool
	)

	flagSet := flag.NewFlagSet("send-message", flag.ExitOnError)
	flagSet.StringVar(&msgType, "type", "", "Action type: 'upgrade' or 'reboot'")
	flagSet.StringVar(&version, "version", "", "Semantic version (e.g. v1.2.3)")
	flagSet.StringVar(&hash, "hash", "", "SHA256 hash of binary (required)")
	flagSet.StringVar(&network, "network", "", "Network identifier (e.g. 'hqz', 'testnet')")
	flagSet.StringVar(&genesis, "genesis", "", "Genesis URL (required for 'reboot')")
	flagSet.StringVar(&requiredBy, "required-by", "", "Unix timestamp deadline (optional for 'reboot')")
	flagSet.BoolVar(&dryRun, "dry-run", false, "Print event instead of sending")
	flagSet.Parse(os.Args[2:])

	// Validate message type
	if msgType != "upgrade" && msgType != "reboot" {
		log.Fatalf("[ERROR] Invalid action type '%s'. Must be 'upgrade' or 'reboot'.", msgType)
	}

	// Validate version
	if version == "" {
		log.Fatal("[ERROR] Version is required.")
	}
	if _, err := semver.NewVersion(version); err != nil {
		log.Fatalf("[ERROR] Invalid semantic version '%s': %v", version, err)
	}

	// Validate required fields
	if hash == "" {
		log.Fatal("[ERROR] Hash is required (use --hash flag)")
	}
	if network == "" {
		log.Fatal("[ERROR] Network is required (use --network flag)")
	}

	// Validate genesis for reboot
	if msgType == "reboot" && genesis == "" {
		log.Fatal("[ERROR] Genesis URL is required for reboot messages (use --genesis flag)")
	}

	// Build event tags based on action type
	tags := nostr.Tags{
		{"d", "hyperqube"},
		{"version", version},
		{"hash", hash},
		{"network", network},
		{"action", msgType},
	}

	// Add reboot-specific tags
	if msgType == "reboot" {
		tags = append(tags, nostr.Tag{"genesis_url", genesis})
		if requiredBy != "" {
			tags = append(tags, nostr.Tag{"required_by", requiredBy})
		}
	}

	// Build human-readable content
	var content string
	if msgType == "upgrade" {
		content = fmt.Sprintf("[hypersignal] A HyperQube upgrade has been released for network %s. Please update binary to version %s.",
			network, version)
	} else {
		content = fmt.Sprintf("[hypersignal] A HyperQube reboot for network %s version %s has been scheduled.",
			network, version)
		if requiredBy != "" {
			content += fmt.Sprintf(" Required by timestamp %s.", requiredBy)
		}
	}

	if dryRun {
		log.Println("[DRY RUN] Prepared HyperSignal event (kind 33321):")
		fmt.Printf("Tags: %v\n", tags)
		fmt.Printf("Content: %s\n", content)
		return
	}

	log.Printf("[INFO] Loading keypair from config directory: %s", configDir)
	kp := loadOrCreateKeypair(configDir)
	_, privKey, err := nip19.Decode(kp.Nsec)
	if err != nil {
		log.Fatalf("[ERROR] Invalid private key: %v", err)
	}

	cfg := loadConfig(configDir)
	if len(cfg.Relays) == 0 {
		log.Println("[WARN] No relays configured; message will not be sent.")
		return
	}

	// Create kind 33321 HyperSignal event
	ev := nostr.Event{
		PubKey:    kp.Npub,
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		Kind:      33321,
		Tags:      tags,
		Content:   content,
	}
	if err := ev.Sign(privKey.(string)); err != nil {
		log.Fatalf("[ERROR] Failed to sign event: %v", err)
	}

	log.Printf("[INFO] Created HyperSignal event (kind 33321) for %s action, version %s", msgType, version)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for _, relayURL := range cfg.Relays {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			log.Printf("[INFO] Connecting to relay %s", url)
			r, err := nostr.RelayConnect(ctx, url)
			if err != nil {
				log.Printf("[WARN] Could not connect to relay %s: %v", url, err)
				return
			}
			defer r.Close()

			log.Printf("[INFO] Publishing message to relay %s", url)
			if err := r.Publish(ctx, ev); err != nil {
				log.Printf("[WARN] Failed to publish to relay %s: %v", url, err)
				return
			}

			log.Printf("[INFO] Successfully published message to relay %s", url)
		}(relayURL)
	}

	wg.Wait()
	log.Println("[INFO] Finished publishing message to all configured relays.")
}
