package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

// CandidateAction holds details of a potential action to perform
type CandidateAction struct {
	Version        *semver.Version // Parsed semantic version
	Type           string          // "upgrade" or "reboot"
	Key            string          // Unique history key
	Genesis        string          // Genesis URL for reboot, empty for upgrade
	Hash           string          // SHA256 hash of binary
	Network        string          // Network identifier (e.g., "hqz")
	OriginalPubkey string          // Pubkey of dev who issued the signal (for kind=3333 reference)
}

// getTagValue returns the value of the first tag with the given name, or empty string if not found
func getTagValue(event *nostr.Event, tagName string) string {
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == tagName {
			return tag[1]
		}
	}
	return ""
}

// hasTag returns true if the event has a tag with the given name
func hasTag(event *nostr.Event, tagName string) bool {
	for _, tag := range event.Tags {
		if len(tag) > 0 && tag[0] == tagName {
			return true
		}
	}
	return false
}

// checkAndExecuteQuorum checks if any action has reached quorum and executes it
// This function is called periodically by the quorum check ticker
func checkAndExecuteQuorum(
	actionsMux *sync.RWMutex,
	actions map[string]*CandidateAction,
	votes map[string]map[string]bool,
	config *Config,
	history *History,
	keypair *Keypair,
	dryRun bool,
) {
	actionsMux.Lock()
	defer actionsMux.Unlock()

	// Select the latest semver action meeting quorum and not already in history
	var latest *CandidateAction
	for _, a := range actions {
		if history.Has(a.Key) {
			continue // skip already acted on
		}

		voteCount := 0
		if vset, ok := votes[a.Key]; ok {
			voteCount = len(vset)
		}

		if voteCount < config.Quorum {
			log.Printf("[DEBUG] Action %s has %d/%d votes (below quorum)", a.Key, voteCount, config.Quorum)
			continue
		}

		if latest == nil || a.Version.GreaterThan(latest.Version) {
			latest = a
		}
	}

	if latest == nil {
		return // No action meeting quorum
	}

	log.Printf("[INFO] Selected action %s with version %s and %d votes",
		latest.Key, latest.Version.Original(), len(votes[latest.Key]))

	switch latest.Type {
	case "upgrade":
		log.Printf("[UPGRADE ACTION] Version: %s", latest.Version.Original())
	case "reboot":
		log.Printf("[REBOOT ACTION] Version: %s Genesis: %s", latest.Version.Original(), latest.Genesis)
	}

	if !dryRun {
		// Build kind=3333 QubeManager status event
		// Use config values for network and node_id
		tags := nostr.Tags{
			{"a", fmt.Sprintf("33321:%s:hyperqube", latest.OriginalPubkey)},
			{"p", latest.OriginalPubkey},
			{"version", latest.Version.Original()},
			{"network", config.Network},
			{"action", latest.Type},
			{"status", "success"},
			{"node_id", config.NodeID},
			{"action_at", fmt.Sprintf("%d", time.Now().Unix())},
		}

		// Build human-readable content
		content := fmt.Sprintf("[qube-manager] The %s to version %s has been successful on node %s.",
			latest.Type, latest.Version.Original(), config.NodeID)

		doneEvent := nostr.Event{
			PubKey:    keypair.Npub,
			CreatedAt: nostr.Timestamp(time.Now().Unix()),
			Kind:      3333,
			Tags:      tags,
			Content:   content,
		}

		_, priv, err := nip19.Decode(keypair.Nsec)
		if err != nil {
			log.Printf("[ERROR] Invalid private key: %v", err)
			return
		}

		if err := doneEvent.Sign(priv.(string)); err != nil {
			log.Printf("[ERROR] Error signing status event: %v", err)
			return
		}

		log.Printf("[INFO] Publishing kind=3333 status event for action %s to %d relays", latest.Key, len(config.Relays))

		for _, r := range config.Relays {
			go func(url string) {
				log.Printf("[INFO] Publishing to relay %s", url)
				if relay, err := nostr.RelayConnect(context.Background(), url); err == nil {
					_ = relay.Publish(context.Background(), doneEvent)
				} else {
					log.Printf("[WARN] Relay publish error (%s): %v", url, err)
				}
			}(r)
		}

		history.Add(latest.Key)
		if err := history.Save(); err != nil {
			log.Printf("[WARN] Error saving history: %v", err)
		} else {
			log.Printf("[INFO] Action %s saved to history", latest.Key)
		}
	} else {
		log.Println("[INFO] Dry run - not saving action to history.")
	}
}

func main() {
	// Command-line flags
	var (
		dryRun     = flag.Bool("dry-run", false, "Perform a trial run without saving actions")
		configDir  = flag.String("config-dir", filepath.Join(os.Getenv("HOME"), ".qube-manager"), "Configuration directory")
		verbose    = flag.Bool("verbose", false, "Enable verbose logging including go-nostr logs")
		showVersion = flag.Bool("version", false, "Show version information and exit")
	)
	flag.Parse()

	// Handle version flag
	if *showVersion {
		fmt.Println(VersionString())
		os.Exit(0)
	}

	log.Printf("[INFO] Starting Qube Manager %s", Version)

	if err := os.MkdirAll(*configDir, 0755); err != nil {
		log.Fatalf("[ERROR] Failed to create config directory: %v", err)
	} else {
		log.Printf("[INFO] Ensured config directory exists at %s", *configDir)
	}

	// Setup logging to file and stdout
	setupLogging(*configDir)

	if *dryRun {
		log.Println("[INFO] Running in dry-run mode")
	}
	if *verbose {
		log.Println("[INFO] Verbose logging enabled")
	}

	log.Println("[INFO] Loading or creating keypair")
	keypair := loadOrCreateKeypair(*configDir)
	_, _, err := nip19.Decode(keypair.Nsec)
	if err != nil {
		log.Fatalf("[ERROR] Invalid private key in config: %v", err)
	}

	// Suppress go-nostr info logs like "filter doesn't match"
	configureNostrLogging(*verbose)
	log.Println("[INFO] Nostr logging configured")

	if len(os.Args) > 1 && os.Args[1] == "send-message" {
		log.Println("[INFO] Handling 'send-message' command")
		sendMessageCLI(*configDir)
		return
	}

	// Load configuration and history from files
	config := loadConfig(*configDir)
	history := loadHistory(*configDir)

	log.Printf("[INFO] Loaded config: %d relays, %d follows, quorum=%d",
		len(config.Relays), len(config.Follows), config.Quorum)

	// Context for graceful shutdown (no timeout - long-running daemon)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("[INFO] Received shutdown signal (%v), cleaning up...", sig)
		cancel()
	}()

	// Map to hold candidate actions keyed by unique history keys
	actions := make(map[string]*CandidateAction)

	// Map of action key -> set of pubkeys that voted for this action
	votes := make(map[string]map[string]bool)

	// Track latest signal from each dev for single active message model
	// Map: dev_pubkey -> latest created_at timestamp
	latestSignal := make(map[string]nostr.Timestamp)

	// Track which action key each dev's latest signal created
	// Map: dev_pubkey -> action_key
	signalActionMap := make(map[string]string)

	// Mutex for thread-safe access to actions and votes maps
	var actionsMux sync.RWMutex

	// Start periodic quorum check ticker (runs every 60 seconds)
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				log.Printf("[DEBUG] Running periodic quorum check...")
				checkAndExecuteQuorum(&actionsMux, actions, votes, &config, history, &keypair, *dryRun)
			case <-ctx.Done():
				log.Printf("[INFO] Quorum checker goroutine shutting down")
				return
			}
		}
	}()

	log.Printf("[INFO] Started quorum check ticker (interval: 60s)")

	// Decode all npubs to hex pubkeys for filtering
	hexFollows := make([]string, 0, len(config.Follows))
	for _, npub := range config.Follows {
		kind, pubkeyAny, err := nip19.Decode(npub)
		if err != nil {
			log.Printf("[WARN] Skipping invalid npub (%s): %v", npub, err)
			continue
		}
		if kind != "npub" {
			log.Printf("[WARN] Expected npub but got %s: %s", kind, npub)
			continue
		}
		pubkey, ok := pubkeyAny.(string)
		if !ok {
			log.Printf("[WARN] Unexpected pubkey format for %s: %v", npub, pubkeyAny)
			continue
		}
		hexFollows = append(hexFollows, pubkey)
	}
	log.Printf("[INFO] Decoded %d valid npubs for following", len(hexFollows))

	// Decode private key for NIP-42 authentication
	_, privKey, err := nip19.Decode(keypair.Nsec)
	if err != nil {
		log.Fatalf("[ERROR] Failed to decode private key: %v", err)
	}

	// Create auth handler for NIP-42 authentication
	authHandler := nostr.WithAuthHandler(func(authCtx context.Context, authEvent nostr.RelayEvent) error {
		evt := authEvent.Event
		if *verbose {
			log.Printf("[DEBUG] Relay %s requested auth, signing challenge", authEvent.Relay.URL)
			log.Printf("[DEBUG] AUTH challenge tags: %v", evt.Tags)
		}
		if err := evt.Sign(privKey.(string)); err != nil {
			log.Printf("[ERROR] Failed to sign AUTH event for %s: %v", authEvent.Relay.URL, err)
			return err
		}
		log.Printf("[INFO] Authenticated with relay %s", authEvent.Relay.URL)
		return nil
	})

	// Create SimplePool with auth handler for proper NIP-42 support
	pool := nostr.NewSimplePool(ctx, authHandler)

	// Subscribe to kind=33321 (HyperSignal) events from followed pubkeys across all relays
	filters := nostr.Filters{{
		Authors: hexFollows,
		Kinds:   []int{33321},
		Tags:    nostr.TagMap{"d": []string{"hyperqube"}},
	}}

	log.Printf("[INFO] Subscribing to %d relay(s) for kind=33321 events", len(config.Relays))
	events := pool.SubMany(ctx, config.Relays, filters)

	// Read events from all relays
	for relayEvent := range events {
		select {
		case <-ctx.Done():
			log.Printf("[INFO] Context cancelled, stopping event processing")
			return
		default:
		}

		ev := relayEvent.Event

		// Validate required tags
		dTag := getTagValue(ev, "d")
		if dTag != "hyperqube" {
			if *verbose {
				log.Printf("[DEBUG] Skipping event with wrong d tag: %s", dTag)
			}
			continue
		}

		// Extract required tags
		version := getTagValue(ev, "version")
		hash := getTagValue(ev, "hash")
		network := getTagValue(ev, "network")
		action := getTagValue(ev, "action")

		// Validate required tags are present
		if version == "" || hash == "" || network == "" || action == "" {
			if *verbose {
				log.Printf("[DEBUG] Skipping event with missing required tags (version=%s, hash=%s, network=%s, action=%s)",
					version, hash, network, action)
			}
			continue
		}

		// Network filtering: only process events for our configured network
		if network != config.Network {
			if *verbose {
				log.Printf("[DEBUG] Skipping event for network %s (we are %s)",
					network, config.Network)
			}
			continue
		}

		// Parse semantic version
		v, err := semver.NewVersion(version)
		if err != nil {
			log.Printf("[WARN] Invalid semantic version: %s", version)
			continue
		}

		// Lock for writing to actions/votes maps
		actionsMux.Lock()

		// Single active message model: Check if this is a newer signal from this dev
		if prevTimestamp, exists := latestSignal[ev.PubKey]; exists {
			if ev.CreatedAt > prevTimestamp {
				// This is a newer signal from the same dev - clear old votes
				if oldActionKey, hasOldAction := signalActionMap[ev.PubKey]; hasOldAction {
					// Remove this dev's vote from the old action
					if oldVotes, oldVotesExist := votes[oldActionKey]; oldVotesExist {
						delete(oldVotes, ev.PubKey)
						log.Printf("[INFO] Cleared vote from pubkey %s for old action %s (superseded by newer signal)",
							ev.PubKey[:8]+"...", oldActionKey)
					}
				}
			} else {
				// This signal is older than what we've already seen from this dev - ignore it
				actionsMux.Unlock()
				if *verbose {
					log.Printf("[DEBUG] Ignoring older signal from pubkey %s (timestamp %d < %d)",
						ev.PubKey[:8]+"...", ev.CreatedAt, prevTimestamp)
				}
				continue
			}
		}

		switch action {
		case "upgrade":
			key := fmt.Sprintf("upgrade:%s", v.Original())
			actionStruct, exists := actions[key]
			if !exists {
				actionStruct = &CandidateAction{
					Type:           "upgrade",
					Version:        v,
					Key:            key,
					Hash:           hash,
					Network:        network,
					OriginalPubkey: ev.PubKey,
				}
				actions[key] = actionStruct
			}

			if votes[key] == nil {
				votes[key] = make(map[string]bool)
			}
			votes[key][ev.PubKey] = true

			// Update tracking for single active message model
			latestSignal[ev.PubKey] = ev.CreatedAt
			signalActionMap[ev.PubKey] = key

			log.Printf("[INFO] Parsed upgrade signal: version=%s network=%s hash=%s pubkey=%s",
				v.Original(), network, hash[:8]+"...", ev.PubKey[:8]+"...")

		case "reboot":
			genesisURL := getTagValue(ev, "genesis_url")
			if genesisURL == "" {
				actionsMux.Unlock()
				log.Printf("[WARN] Reboot action missing genesis_url tag")
				continue
			}

			if _, err := url.ParseRequestURI(genesisURL); err != nil {
				actionsMux.Unlock()
				log.Printf("[WARN] Invalid genesis URL in reboot: %s", genesisURL)
				continue
			}

			key := fmt.Sprintf("reboot:%s:%s", v.Original(), genesisURL)
			actionStruct, exists := actions[key]
			if !exists {
				actionStruct = &CandidateAction{
					Type:           "reboot",
					Version:        v,
					Key:            key,
					Genesis:        genesisURL,
					Hash:           hash,
					Network:        network,
					OriginalPubkey: ev.PubKey,
				}
				actions[key] = actionStruct
			}

			if votes[key] == nil {
				votes[key] = make(map[string]bool)
			}
			votes[key][ev.PubKey] = true

			// Update tracking for single active message model
			latestSignal[ev.PubKey] = ev.CreatedAt
			signalActionMap[ev.PubKey] = key

			log.Printf("[INFO] Parsed reboot signal: version=%s network=%s genesis=%s hash=%s pubkey=%s",
				v.Original(), network, genesisURL, hash[:8]+"...", ev.PubKey[:8]+"...")

		default:
			if *verbose {
				log.Printf("[DEBUG] Ignoring event with unknown action type: %s", action)
			}
		}

		actionsMux.Unlock()
	}

	log.Printf("[INFO] Event stream ended")
	log.Printf("[INFO] Qube Manager shutting down cleanly")
}
