package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
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

func main() {
	// Command-line flags
	var (
		dryRun    = flag.Bool("dry-run", false, "Perform a trial run without saving actions")
		configDir = flag.String("config-dir", filepath.Join(os.Getenv("HOME"), ".qube-manager"), "Configuration directory")
		verbose   = flag.Bool("verbose", false, "Enable verbose logging including go-nostr logs")
	)
	flag.Parse()

	log.Printf("[INFO] Starting Qube Manager")

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

	// Context with timeout to avoid hanging connections
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Map to hold candidate actions keyed by unique history keys
	actions := make(map[string]*CandidateAction)

	// Map of action key -> set of pubkeys that voted for this action
	votes := make(map[string]map[string]bool)

	// Connect to each relay and subscribe to relevant events
	for _, relayURL := range config.Relays {
		start := time.Now()
		log.Printf("[INFO] Connecting to relay: %s", relayURL)
		relay, err := nostr.RelayConnect(ctx, relayURL)
		if err != nil {
			log.Printf("[WARN] Failed to connect to relay %s: %v (took %v)", relayURL, err, time.Since(start))
			continue
		}
		log.Printf("[INFO] Connected to relay: %s (took %v)", relayURL, time.Since(start))

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
		log.Printf("[INFO] Relay %s: decoded %d valid npubs for following", relayURL, len(hexFollows))

		// Subscribe to kind=33321 (HyperSignal) events authored by followed pubkeys
		sub, err := relay.Subscribe(ctx, nostr.Filters{{
			Authors: hexFollows,
			Kinds:   []int{33321},
			Tags:    nostr.TagMap{"d": []string{"hyperqube"}},
		}})
		if err != nil {
			log.Printf("[ERROR] Subscription failed on %s: %v", relayURL, err)
			continue
		}
		log.Printf("[INFO] Subscription successful on %s", relayURL)

		// Ensure subscription gets cleaned up
		defer func(relayURL string) {
			log.Printf("[INFO] Closing subscription on %s", relayURL)
			sub.Close()
			log.Printf("[INFO] Subscription on relay %s closed", relayURL)
		}(relayURL)

		// Read events and parse HyperSignal messages from tags
		for ev := range sub.Events {
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

			// Parse semantic version
			v, err := semver.NewVersion(version)
			if err != nil {
				log.Printf("[WARN] Invalid semantic version: %s", version)
				continue
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

				log.Printf("[INFO] Parsed upgrade signal: version=%s network=%s hash=%s pubkey=%s",
					v.Original(), network, hash[:8]+"...", ev.PubKey[:8]+"...")

			case "reboot":
				genesisURL := getTagValue(ev, "genesis_url")
				if genesisURL == "" {
					log.Printf("[WARN] Reboot action missing genesis_url tag")
					continue
				}

				if _, err := url.ParseRequestURI(genesisURL); err != nil {
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

				log.Printf("[INFO] Parsed reboot signal: version=%s network=%s genesis=%s hash=%s pubkey=%s",
					v.Original(), network, genesisURL, hash[:8]+"...", ev.PubKey[:8]+"...")

			default:
				if *verbose {
					log.Printf("[DEBUG] Ignoring event with unknown action type: %s", action)
				}
			}
		}
	}

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
			log.Printf("[INFO] Skipping action %s - votes %d/%d (below quorum)", a.Key, voteCount, config.Quorum)
			continue
		}

		if latest == nil || a.Version.GreaterThan(latest.Version) {
			latest = a
		}
	}

	if latest != nil {
		log.Printf("[INFO] Selected action %s with version %s and %d votes",
			latest.Key, latest.Version.Original(), len(votes[latest.Key]))

		switch latest.Type {
		case "upgrade":
			log.Printf("[UPGRADE ACTION] Version: %s", latest.Version.Original())
		case "reboot":
			log.Printf("[REBOOT ACTION] Version: %s Genesis: %s", latest.Version.Original(), latest.Genesis)
		}

		if !*dryRun {
			// Build kind=3333 QubeManager status event
			// Note: network and node_id will use placeholder values until Phase 4
			network := latest.Network
			nodeID := "qube-node-" + keypair.Npub[:8] // Temporary placeholder

			// Build tags for kind=3333 event
			tags := nostr.Tags{
				{"a", fmt.Sprintf("33321:%s:hyperqube", latest.OriginalPubkey)},
				{"p", latest.OriginalPubkey},
				{"version", latest.Version.Original()},
				{"network", network},
				{"action", latest.Type},
				{"status", "success"},
				{"node_id", nodeID},
				{"action_at", fmt.Sprintf("%d", time.Now().Unix())},
			}

			// Build human-readable content
			content := fmt.Sprintf("[qube-manager] The %s to version %s has been successful on node %s.",
				latest.Type, latest.Version.Original(), nodeID)

			doneEvent := nostr.Event{
				PubKey:    keypair.Npub,
				CreatedAt: nostr.Timestamp(time.Now().Unix()),
				Kind:      3333,
				Tags:      tags,
				Content:   content,
			}

			_, priv, err := nip19.Decode(keypair.Nsec)
			if err != nil {
				log.Fatalf("[ERROR] Invalid private key: %v", err)
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
	} else {
		log.Println("[INFO] No new eligible actions to perform.")
	}
}
