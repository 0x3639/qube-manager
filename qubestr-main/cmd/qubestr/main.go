package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/coinselor/qubestr/internal/handlers"
	"github.com/fiatjaf/eventstore/postgresql"
	"github.com/fiatjaf/khatru"
	"github.com/joho/godotenv"
)

func init() {
	godotenv.Load()
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func main() {
	relay := khatru.NewRelay()

	dbURL := fmt.Sprintf(
		"postgresql://%s:%s@%s:%s/%s?sslmode=%s",
		getEnv("DB_USER", "postgres"),
		getEnv("DB_PASSWORD", "postgres"),
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_NAME", "qubestr"),
		getEnv("DB_SSLMODE", "disable"),
	)

	db := postgresql.PostgresBackend{
		DatabaseURL: dbURL,
	}

	queryLimitStr := getEnv("DB_QUERY_LIMIT", "100")
	queryLimit, err := strconv.Atoi(queryLimitStr)
	if err != nil {
		log.Printf("Warning: Invalid DB_QUERY_LIMIT value '%s', using default 100. Error: %v", queryLimitStr, err)
		queryLimit = 100
	}
	db.QueryLimit = queryLimit

	keepRecentEventsStr := getEnv("DB_KEEP_RECENT_EVENTS", "true")
	keepRecentEvents, err := strconv.ParseBool(keepRecentEventsStr)
	if err != nil {
		log.Printf("Warning: Invalid DB_KEEP_RECENT_EVENTS value '%s', using default true. Error: %v", keepRecentEventsStr, err)
		keepRecentEvents = true
	}
	db.KeepRecentEvents = keepRecentEvents

	if err := db.Init(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	relay.StoreEvent = append(relay.StoreEvent, db.SaveEvent)
	relay.QueryEvents = append(relay.QueryEvents, db.QueryEvents)
	relay.DeleteEvent = append(relay.DeleteEvent, db.DeleteEvent)
	relay.ReplaceEvent = append(relay.ReplaceEvent, db.ReplaceEvent)
	relay.CountEvents = append(relay.CountEvents, db.CountEvents)

	// Relay information (NIP-11)
	relay.Info.Name = "Qubestr: A Specialized Nostr Relay for HyperQube Network"
	relay.Info.Description = "Supports HyperQube's custom events (kinds 33321 and 3333) for managing HyperQube nodes."
	relay.Info.PubKey = getEnv("RELAY_ADMIN_PUBKEY", "")
	relay.Info.SupportedNIPs = []any{1, 11, 33, 42}

	// Authentication (NIP-42)
	relay.OnConnect = append(relay.OnConnect, func(ctx context.Context) {
		khatru.RequestAuth(ctx)
	})

	relay.RejectEvent = append(relay.RejectEvent,
		handlers.ValidateKind,
		handlers.ValidateHyperSignalEvent,
		handlers.ValidateQubeManagerEvent,
	)

	relay.RejectFilter = append(relay.RejectFilter, handlers.RequireAuth)

	mux := relay.Router()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<h1>Qubestr Nostr Relay</h1>
<p>Specialized relay for hyperqube custom events (kinds 33321 and 3333)</p>`)
	})

	port := getEnv("PORT", "3334")
	host := getEnv("HOST", "0.0.0.0")
	listenAddr := fmt.Sprintf("%s:%s", host, port)

	fmt.Printf("Starting Qubestr relay on %s\n", listenAddr)
	if err := http.ListenAndServe(listenAddr, relay); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
