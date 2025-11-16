package handlers

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/fiatjaf/khatru"
	"github.com/nbd-wtf/go-nostr"
)

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func ValidateKind(ctx context.Context, event *nostr.Event) (bool, string) {
	if event.Kind != 33321 && event.Kind != 3333 {
		return true, fmt.Sprintf("unsupported kind: %d", event.Kind)
	}
	return false, ""
}

func ValidateHyperSignalEvent(ctx context.Context, event *nostr.Event) (bool, string) {
	if event.Kind == 33321 {
		authenticatedUserPubkey := khatru.GetAuthed(ctx)
		if authenticatedUserPubkey == "" {
			message := "auth-required: publishing Kind 33321 requires authentication"
			log.Printf("Event rejected. ID: %s, Kind: %d, Pubkey: %s, Reason: %s", event.ID, event.Kind, event.PubKey, message)
			return true, message
		}
		authorizedPubkeys := strings.Split(getEnv("AUTHORIZED_PUBKEYS", ""), ",")
		isAuthorized := false
		for _, pubkey := range authorizedPubkeys {
			if authenticatedUserPubkey == pubkey {
				isAuthorized = true
				break
			}
		}
		if !isAuthorized {
			message := "restricted: your public key is not authorized to publish Kind 33321 events"
			log.Printf("Event rejected. ID: %s, Kind: %d, Pubkey: %s, Reason: %s", event.ID, event.Kind, event.PubKey, message)
			return true, message
		}

		if !hasTagWithValue(event, "d", "hyperqube") {
			message := "hyperqube: missing required 'd' tag with value 'hyperqube'"
			log.Printf("Event rejected. ID: %s, Kind: %d, Pubkey: %s, Reason: %s", event.ID, event.Kind, event.PubKey, message)
			return true, message
		}

		requiredTags := []string{"version", "hash", "network", "action"}
		for _, tag := range requiredTags {
			if !hasTag(event, tag) {
				message := fmt.Sprintf("hyperqube: missing required '%s' tag", tag)
				log.Printf("Event rejected. ID: %s, Kind: %d, Pubkey: %s, Reason: %s", event.ID, event.Kind, event.PubKey, message)
				return true, message
			}
		}

		actionTag := getTagValue(event, "action")
		if actionTag != "upgrade" && actionTag != "reboot" {
			message := "hyperqube: 'action' tag must be either 'upgrade' or 'reboot'"
			log.Printf("Event rejected. ID: %s, Kind: %d, Pubkey: %s, Reason: %s", event.ID, event.Kind, event.PubKey, message)
			return true, message
		}

		if actionTag == "reboot" && (!hasTag(event, "genesis_url") || !hasTag(event, "required_by")) {
			message := "hyperqube: 'reboot' action requires 'genesis_url' and 'required_by' tags"
			log.Printf("Event rejected. ID: %s, Kind: %d, Pubkey: %s, Reason: %s", event.ID, event.Kind, event.PubKey, message)
			return true, message
		}

		if len(event.Content) == 0 {
			message := "hyperqube: content must be a human-readable string"
			log.Printf("Event rejected. ID: %s, Kind: %d, Pubkey: %s, Reason: %s", event.ID, event.Kind, event.PubKey, message)
			return true, message
		}
	}
	return false, ""
}

func ValidateQubeManagerEvent(ctx context.Context, event *nostr.Event) (bool, string) {
	if event.Kind == 3333 {
		if khatru.GetAuthed(ctx) == "" {
			message := "auth-required: publishing Kind 3333 requires authentication"
			log.Printf("Event rejected. ID: %s, Kind: %d, Pubkey: %s, Reason: %s", event.ID, event.Kind, event.PubKey, message)
			return true, message
		}

		requiredTags := []string{"a", "version", "network", "action", "status", "node_id", "action_at"}
		for _, tag := range requiredTags {
			if !hasTag(event, tag) {
				message := fmt.Sprintf("qube-manager: missing required '%s' tag", tag)
				log.Printf("Event rejected. ID: %s, Kind: %d, Pubkey: %s, Reason: %s", event.ID, event.Kind, event.PubKey, message)
				return true, message
			}
		}

		aTagValue := getTagValue(event, "a")
		parts := strings.Split(aTagValue, ":")
		if len(parts) != 3 || parts[0] != "33321" || len(parts[1]) != 64 || parts[2] != "hyperqube" {
			message := "qube-manager: invalid 'a' tag format (should be '33321:<64_hex_pubkey>:hyperqube')"
			log.Printf("Event rejected. ID: %s, Kind: %d, Pubkey: %s, Reason: %s", event.ID, event.Kind, event.PubKey, message)
			return true, message
		}

		actionTag := getTagValue(event, "action")
		if actionTag != "upgrade" && actionTag != "reboot" {
			message := "qube-manager: 'action' tag must be either 'upgrade' or 'reboot'"
			log.Printf("Event rejected. ID: %s, Kind: %d, Pubkey: %s, Reason: %s", event.ID, event.Kind, event.PubKey, message)
			return true, message
		}

		statusTag := getTagValue(event, "status")
		if statusTag != "success" && statusTag != "failure" {
			message := "qube-manager: 'status' tag should be 'success' or 'failure'"
			log.Printf("Event rejected. ID: %s, Kind: %d, Pubkey: %s, Reason: %s", event.ID, event.Kind, event.PubKey, message)
			return true, message
		}

		if statusTag == "failure" && !hasTag(event, "error") {
			message := "qube-manager: 'failure' status requires 'error' tag"
			log.Printf("Event rejected. ID: %s, Kind: %d, Pubkey: %s, Reason: %s", event.ID, event.Kind, event.PubKey, message)
			return true, message
		}

		if len(event.Content) == 0 {
			message := "qube-manager: content must be a human-readable string"
			log.Printf("Event rejected. ID: %s, Kind: %d, Pubkey: %s, Reason: %s", event.ID, event.Kind, event.PubKey, message)
			return true, message
		}
	}
	return false, ""
}

func RequireAuth(ctx context.Context, filter nostr.Filter) (bool, string) {
	if khatru.GetAuthed(ctx) == "" {
		return true, "auth-required: this relay requires authentication to read events"
	}
	return false, ""
}

// --- Helper functions ---

func hasTag(event *nostr.Event, name string) bool {
	for _, tag := range event.Tags {
		if len(tag) > 0 && tag[0] == name {
			return true
		}
	}
	return false
}

func hasTagWithValue(event *nostr.Event, name string, value string) bool {
	for _, tag := range event.Tags {
		if len(tag) > 1 && tag[0] == name && tag[1] == value {
			return true
		}
	}
	return false
}

func getTagValue(event *nostr.Event, name string) string {
	for _, tag := range event.Tags {
		if len(tag) > 1 && tag[0] == name {
			return tag[1]
		}
	}
	return ""
}
