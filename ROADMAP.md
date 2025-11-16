# Qube-Manager Roadmap: Full Qubestr Compatibility

## Overview

This roadmap tracks the refactoring of qube-manager to achieve full compatibility with the Qubestr relay. The current implementation uses kind=1 JSON-based events and a single-run model. The target implementation uses kind=33321/3333 tag-based events and a long-running daemon architecture.

**Status**: 🔴 Not Started
**Target Completion**: TBD
**Last Updated**: 2025-11-16

---

## Critical Issues Identified

### 🔴 CRITICAL: Event Format Incompatibility
- **Current**: Kind=1 events with JSON in content field
- **Required**: Kind=33321 events with data in tags
- **Impact**: qube-manager and qubestr cannot communicate at all
- **Status**: Not started

### 🔴 CRITICAL: No Real Quorum Coordination
- **Current**: 10-second timeout, single-run model
- **Issue**: Votes arriving after timeout are never counted
- **Required**: Long-running daemon that accumulates votes over time
- **Status**: Not started

### 🟡 HIGH: No Hash Validation
- **Current**: Upgrades executed without verifying binary hash
- **Required**: SHA256 validation before execution
- **Impact**: Security vulnerability
- **Status**: Not started

### 🟡 HIGH: No Vote Persistence
- **Current**: Votes stored in memory only
- **Issue**: Lost on restart/crash
- **Required**: Persist votes to disk
- **Status**: Not started

### 🟡 MEDIUM: No Network Filtering
- **Current**: Cannot distinguish between networks
- **Required**: Only process events for configured network
- **Status**: Not started

---

## Phase 1: Event Format Migration (Kind 1 → Kind 33321/3333)

### 1.1 Add Tag Helper Functions ✅
**File**: `main.go`
**Description**: Create utility functions for parsing Nostr event tags

**Tasks**:
- [x] Add `getTagValue(event *nostr.Event, tagName string) string` function
- [x] Add `hasTag(event *nostr.Event, tagName string) bool` function

**Reference**: qubestr-main/internal/handlers/validation.go:17-20

---

### 1.2 Update Event Subscription ✅
**File**: `main.go:143-148`
**Description**: Subscribe to kind=33321 events instead of kind=1

**Completed Changes**:
```go
sub, err := relay.Subscribe(ctx, nostr.Filters{{
    Authors: hexFollows,
    Kinds:   []int{33321},
    Tags:    nostr.TagMap{"d": []string{"hyperqube"}},
}})
```

**Tasks**:
- [x] Change `Kinds: []int{1}` to `Kinds: []int{33321}`
- [x] Add tag filter for addressable events: `Tags: nostr.TagMap{"d": []string{"hyperqube"}}`

**Reference**: qubestr-main/hyperqube-events.md:19-76

---

### 1.3 Rewrite Event Processing Loop ✅
**File**: `main.go:162-259`
**Description**: Parse event tags instead of JSON content

**Completed Changes**:
- Replaced JSON unmarshal logic with tag parsing using `getTagValue()`
- Extract required tags: `d`, `version`, `hash`, `network`, `action`
- For reboot: extract `genesis_url`
- Validate all required tags exist before processing
- Skip events with missing `d` tag or `d != "hyperqube"`
- Store `hash`, `network`, and `OriginalPubkey` in CandidateAction struct
- Log verbose warnings for invalid/skipped events

**Tasks**:
- [x] Replace JSON unmarshal logic with tag parsing
- [x] Extract required tags: `d`, `version`, `hash`, `network`, `action`
- [x] For reboot: extract `genesis_url`, `required_by`
- [x] Validate all required tags exist before processing
- [x] Skip events with missing `d` tag or `d != "hyperqube"`
- [x] Log warnings for invalid/skipped events

**Reference**: qubestr-main/hyperqube-events.md:48-57

---

### 1.4 Update Message Sending (send-message) ✅
**File**: `messages.go:33-165`
**Description**: Send kind=33321 events with tag-based format

**Completed Changes**:
```go
// Added CLI flags
flagSet.StringVar(&hash, "hash", "", "SHA256 hash of binary (required)")
flagSet.StringVar(&network, "network", "", "Network identifier (required)")
flagSet.StringVar(&requiredBy, "required-by", "", "Unix timestamp deadline (optional for 'reboot')")

// Create kind 33321 event with tags
ev := nostr.Event{
    Kind: 33321,
    Tags: tags,
    Content: content,
    CreatedAt: nostr.Timestamp(time.Now().Unix()),
}
```

**Tasks**:
- [x] Add `--hash` flag to send-message command
- [x] Add `--network` flag to send-message command
- [x] Add `--required-by` flag for reboot deadlines
- [x] Change event kind from 1 to 33321
- [x] Build tags array instead of JSON content
- [x] Move message data from content to tags
- [x] Update content to human-readable description
- [x] For reboot: ensure `genesis_url` and `required_by` tags included

**Reference**: qubestr-main/hyperqube-events.md:77-116

---

### 1.5 Implement Kind 3333 Status Reports ✅
**File**: `main.go:294-350`
**Description**: Publish kind=3333 acknowledgement events instead of kind=1 "done" events

**Completed Changes**:
```go
doneEvent := nostr.Event{
    Kind: 3333,
    Tags: nostr.Tags{
        {"a", fmt.Sprintf("33321:%s:hyperqube", latest.OriginalPubkey)},
        {"p", latest.OriginalPubkey},
        {"version", latest.Version.Original()},
        {"network", network},
        {"action", latest.Type},
        {"status", "success"},
        {"node_id", nodeID},
        {"action_at", fmt.Sprintf("%d", time.Now().Unix())},
    },
    Content: fmt.Sprintf("[qube-manager] The %s to version %s has been successful on node %s.",
                         latest.Type, latest.Version.Original(), nodeID),
}
```

**Tasks**:
- [x] Change from kind=1 to kind=3333
- [x] Add `a` tag referencing the 33321 event (format: `33321:dev_pubkey:hyperqube`)
- [x] Add `p` tag referencing dev pubkey
- [x] Add `version`, `network`, `action`, `status` tags
- [x] Add `node_id` and `action_at` tags
- [x] Track original dev pubkey from signal event
- [x] Update content to human-readable acknowledgement

**Note**: Using temporary placeholder for node_id until Phase 4 config changes

**Reference**: qubestr-main/hyperqube-events.md:118-196

---

## Phase 2: Long-Running Daemon Architecture

### 2.1 Remove Timeout, Add Graceful Shutdown ✅
**File**: `main.go:217-228`
**Description**: Convert from single-run to persistent daemon

**Current Code**:
```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
```

**Target Code**:
```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// Signal handling
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

go func() {
    <-sigChan
    log.Printf("[INFO] Received shutdown signal, cleaning up...")
    cancel()
}()
```

**Tasks**:
- [x] Replace `WithTimeout` with `WithCancel`
- [x] Add signal handler for SIGINT/SIGTERM
- [x] Implement graceful shutdown (context cancellation)
- [x] Add cleanup logging

**Completed Changes**:
```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
go func() {
    sig := <-sigChan
    log.Printf("[INFO] Received shutdown signal (%v), cleaning up...", sig)
    cancel()
}()
```

---

### 2.2 Implement Periodic Quorum Check ✅
**File**: `main.go:52-164` (checkAndExecuteQuorum function)
**Description**: Check quorum every 60 seconds instead of once at program end

**Tasks**:
- [x] Extract quorum selection logic into `checkAndExecuteQuorum()` function
- [x] Add mutex for thread-safe access to `actions` and `votes` maps
- [x] Create ticker that runs every 60 seconds
- [x] Call `checkAndExecuteQuorum()` on each tick
- [x] Keep function logic: select highest version, check history, execute if quorum met
- [x] Log each quorum check attempt

**Completed Changes**:
```go
var actionsMux sync.RWMutex

ticker := time.NewTicker(60 * time.Second)
go func() {
    for {
        select {
        case <-ticker.C:
            checkAndExecuteQuorum(&actionsMux, actions, votes, &config, history, &keypair, *dryRun)
        case <-ctx.Done():
            return
        }
    }
}()
```

**Pseudocode**:
```go
var actionsMux sync.RWMutex

func checkAndExecuteQuorum() {
    actionsMux.Lock()
    defer actionsMux.Unlock()

    // Existing quorum selection logic
    // ...
}

// In main:
ticker := time.NewTicker(60 * time.Second)
go func() {
    for range ticker.C {
        checkAndExecuteQuorum()
    }
}()
```

---

### 2.3 Restructure Main Event Loop ✅
**File**: `main.go:258-431`
**Description**: Separate vote processing from quorum checking

**Completed Flow**:
1. Connect to relays (in parallel goroutines)
2. Goroutine per relay: Process votes from subscription (runs indefinitely)
3. Ticker goroutine: Check quorum every 60s (runs indefinitely)
4. Signal handler goroutine: Wait for SIGINT/SIGTERM
5. On shutdown: cancel context, all goroutines exit cleanly

**Tasks**:
- [x] Move relay connections and event processing into separate goroutines
- [x] Move quorum checking into ticker goroutine
- [x] Add WaitGroup to wait for all relay goroutines
- [x] Ensure all goroutines exit on context cancellation
- [x] Add proper synchronization with mutex for actions/votes maps
- [x] Add context checks in event processing loop
- [x] Remove old single-run quorum check code

**Completed Changes**:
```go
var wg sync.WaitGroup
for _, relayURL := range config.Relays {
    wg.Add(1)
    go func(relayURL string) {
        defer wg.Done()
        // Connect, subscribe, process events
        for ev := range sub.Events {
            select {
            case <-ctx.Done():
                return
            default:
            }
            // Parse event, lock mutex, update votes
            actionsMux.Lock()
            // ... update actions/votes ...
            actionsMux.Unlock()
        }
    }(relayURL)
}

wg.Wait() // Wait for shutdown
```

---

### 2.4 Add Vote Persistence ⬜
**File**: New `votes.go` or extend `history.go`
**Description**: Save vote state to disk, load on startup

**Data Structure**:
```yaml
# votes.yaml
votes:
  "upgrade:v1.0.0":
    - "pubkey1hex..."
    - "pubkey2hex..."
  "reboot:v2.0.0:https://example.com/genesis.json":
    - "pubkey3hex..."
    - "pubkey4hex..."
    - "pubkey5hex..."
```

**Tasks**:
- [ ] Create `Votes` struct with `Load()`, `Save()`, `AddVote()`, `GetVotes()` methods
- [ ] Save votes.yaml in config directory (~/.qube-manager/votes.yaml)
- [ ] Load votes on startup
- [ ] Save votes to disk after each new vote received
- [ ] Clear votes for action after it's executed and added to history
- [ ] Handle file I/O errors gracefully

**Alternative**: Extend history.yaml to include vote tracking

---

## Phase 3: Single Active Message Model

### 3.1 Implement Vote Clearing Logic ✅
**File**: `main.go:358-379, 402-445` (event processing)
**Description**: When new HyperSignal from dev arrives, clear votes for old version

**Behavior**:
- Track `latestTimestamp[devPubkey] = created_at` for each dev
- When new event arrives from dev:
  - If `created_at > latestTimestamp[devPubkey]`:
    - Clear votes for all actions from that dev's old events
    - Update `latestTimestamp[devPubkey]`
    - Process new event normally

**Tasks**:
- [x] Add `latestSignal map[string]nostr.Timestamp` to track newest event per dev
- [x] Add `signalActionMap map[string]string` to track which action each dev's signal created
- [x] On new event: check if newer than previous from same dev
- [x] If newer: clear votes for old action key from that dev
- [x] Update tracking maps after processing each vote
- [x] Ignore events with older timestamps from same dev

**Completed Changes**:
```go
// Track latest signal from each dev
latestSignal := make(map[string]nostr.Timestamp)
signalActionMap := make(map[string]string)

// In event processing loop:
if prevTimestamp, exists := latestSignal[ev.PubKey]; exists {
    if ev.CreatedAt > prevTimestamp {
        // Clear old votes from this dev
        if oldActionKey, hasOldAction := signalActionMap[ev.PubKey]; hasOldAction {
            delete(votes[oldActionKey], ev.PubKey)
        }
    } else {
        continue // Ignore older signal
    }
}

// After processing vote:
latestSignal[ev.PubKey] = ev.CreatedAt
signalActionMap[ev.PubKey] = key
```

**Rationale**: Kind 33321 is "addressable/replaceable" - newer events supersede older ones

**Reference**: qubestr-main/hyperqube-events.md:22-28

---

### 3.2 Handle Multiple Devs Voting on Different Versions ✅
**File**: `main.go:checkAndExecuteQuorum` (quorum check)
**Description**: Ensure highest version wins when quorum is met

**Scenario**:
- Dev1 signals: upgrade:v1.0.0
- Dev2 signals: upgrade:v2.0.0
- Dev3, Dev4, Dev5 must choose which to vote on

**Current Behavior**: Both accumulate votes independently
**Target Behavior**:
- Each dev's latest signal supersedes their previous
- Quorum check selects highest version with quorum
- Once executed, all votes cleared

**Tasks**:
- [x] Document expected behavior in comments
- [x] Verify quorum selection picks highest version (already implemented in checkAndExecuteQuorum)
- [x] Behavior confirmed: highest semantic version with quorum wins

**Implementation**: Already correctly implemented in Phase 2's `checkAndExecuteQuorum()` function:
```go
if latest == nil || a.Version.GreaterThan(latest.Version) {
    latest = a
}
```

**Note**: This behavior was already correct from Phase 1, no changes needed

---

## Phase 4: Security & Configuration

### 4.1 Add Network Configuration ✅
**File**: `config.go:16-23, 25-33, 42-50, 76-97`
**Description**: Add network and node_id to config

**Current Config Struct** (config.go:8-14):
```go
type Config struct {
    Relays     []string `yaml:"relays"`
    Follows    []string `yaml:"follows"`
    Quorum     int      `yaml:"quorum"`
    ConfigPath string   `yaml:"-"`
}
```

**Target Config Struct**:
```go
type Config struct {
    Relays     []string `yaml:"relays"`
    Follows    []string `yaml:"follows"`
    Quorum     int      `yaml:"quorum"`
    Network    string   `yaml:"network"`     // NEW: e.g., "hqz", "testnet"
    NodeID     string   `yaml:"node_id"`     // NEW: unique node identifier
    ConfigPath string   `yaml:"-"`
}
```

**Default Config** (config.go:35-47):
```yaml
relays:
  - wss://relay.damus.io
  - wss://relay.nostr.band
follows:
  - npub1example...
quorum: 3
network: hqz              # NEW
node_id: node-{random}    # NEW: generate UUID on first run
```

**Tasks**:
- [x] Add `Network` field to Config struct
- [x] Add `NodeID` field to Config struct
- [x] Create `generateNodeID()` function using crypto/rand for UUID generation
- [x] Update default config generation to include network="hqz" and generated node_id
- [x] Handle missing network/node_id in existing configs (auto-add and save)
- [x] Update log output to show network and node_id

**Completed Changes**:
```go
type Config struct {
    Network string `yaml:"network"` // NEW
    NodeID  string `yaml:"node_id"` // NEW
}

func generateNodeID() string {
    // Generates UUID-like: node-xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
}

// Auto-upgrade existing configs
if cfg.Network == "" {
    cfg.Network = "hqz"
    updated = true
}
if cfg.NodeID == "" {
    cfg.NodeID = generateNodeID()
    updated = true
}
```

---

### 4.2 Implement Network Filtering ✅
**File**: `main.go:348-355` (event processing loop)
**Description**: Only process events for configured network

**Tasks**:
- [x] After parsing event tags, extract `network` tag
- [x] Compare against `config.Network`
- [x] Skip events with mismatched network
- [x] Add verbose logging for filtered events
- [x] Update checkAndExecuteQuorum to use config.Network instead of placeholder

**Completed Changes**:
```go
// Network filtering: only process events for our configured network
if network != config.Network {
    if *verbose {
        log.Printf("[DEBUG] Skipping event for network %s (we are %s)",
                   network, config.Network)
    }
    continue
}
```

**Example**:
```go
eventNetwork := getTagValue(ev, "network")
if eventNetwork != config.Network {
    if *verbose {
        log.Printf("[DEBUG] Skipping event for network %s (we are %s)",
                   eventNetwork, config.Network)
    }
    continue
}
```

---

### 4.3 Binary Hash Validation ✅
**File**: `validation.go` (new file)
**Description**: Verify binary SHA256 hash before executing upgrade

**Tasks**:
- [x] Create new file `validation.go`
- [x] Implement `verifyBinaryHash(binaryPath, expectedHash string) error`
  - [x] Open binary file
  - [x] Compute SHA256 hash using crypto/sha256
  - [x] Compare hex-encoded hash with expected
  - [x] Return descriptive error if mismatch
- **Note**: Function implemented and ready for integration when upgrade execution is added

**Function Signature**:
```go
// validation.go
package main

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "io"
    "os"
)

// verifyBinaryHash computes SHA256 of file and compares to expected hash
func verifyBinaryHash(binaryPath, expectedHash string) error {
    // Implementation
}
```

**Integration Point**: main.go:248-325 (action execution)

**Reference**: qubestr-main/hyperqube-events.md:52 (hash tag requirement)

---

### 4.4 Handle Validation Failures ⬜
**File**: `main.go` (action execution)
**Description**: Publish failure status when validation fails
**Status**: Deferred - will implement when actual upgrade/reboot execution is added

**Scenarios to handle**:
1. Hash mismatch
2. Binary not found
3. Download failed
4. Execution error

**Future Tasks**:
- [ ] Wrap action execution in error handling
- [ ] On error: determine failure reason
- [ ] Publish kind=3333 event with `status: failure` and `error: <reason>`
- [ ] Add to history to prevent retry loop
- [ ] Log failure clearly

**Note**: Currently qube-manager logs actions but doesn't execute them. Failure handling
will be added when actual execution logic is implemented.

**Example**:
```go
if err := verifyBinaryHash(binaryPath, latest.Hash); err != nil {
    log.Printf("[ERROR] Hash validation failed: %v", err)
    publishFailureStatus(latest, fmt.Sprintf("hash validation failed: %v", err))
    history.Add(latest.Key)  // Prevent retry
    return
}
```

---

## Phase 5: Testing & Integration

### 5.1 Unit Tests ⬜
**File**: New test files

**Tasks**:
- [ ] `main_test.go`: Test tag parsing helpers
- [ ] `main_test.go`: Test vote accumulation logic
- [ ] `main_test.go`: Test quorum selection (highest version)
- [ ] `main_test.go`: Test vote clearing when new signal arrives
- [ ] `config_test.go`: Test config loading with network/node_id
- [ ] `validation_test.go`: Test hash validation

---

### 5.2 Integration Test with Qubestr ⬜
**Description**: End-to-end test with local qubestr relay

**Setup**:
- [ ] Start qubestr relay locally (docker-compose)
- [ ] Configure test pubkeys in qubestr's `AUTHORIZED_PUBKEYS`
- [ ] Point qube-manager config to `ws://localhost:3334`

**Test Cases**:
- [ ] **TC1**: Send 3/5 kind=33321 events for upgrade:v1.0.0, verify quorum reached
- [ ] **TC2**: Send 2/5 votes for v1.0.0, then 3/5 for v2.0.0, verify v2.0.0 executes
- [ ] **TC3**: Send event with wrong network, verify filtered out
- [ ] **TC4**: Send event with bad hash, verify upgrade rejected
- [ ] **TC5**: Restart qube-manager mid-voting, verify votes restored
- [ ] **TC6**: Verify kind=3333 status event published after execution
- [ ] **TC7**: Verify history prevents re-execution

---

### 5.3 Documentation Updates ✅

**Tasks**:
- [x] Update README.md with new config options
- [x] Update CLAUDE.md with new architecture details
- [x] Add examples for kind=33321/3333 event formats
- [x] Document network configuration
- [x] Document hash validation requirements
- [ ] Add troubleshooting section (deferred)

---

## Phase 6: Future Enhancements (Post-MVP)

### 6.1 NIP-42 Authentication ⬜
**Priority**: Medium (qubestr requires it, but can defer)
**Description**: Implement Nostr authentication protocol

**Tasks**:
- [ ] Research NIP-42 auth challenge/response flow
- [ ] Implement auth handler in relay connection
- [ ] Sign AUTH events with node keypair
- [ ] Handle auth failures gracefully
- [ ] Test with qubestr's auth requirement

**Reference**: qubestr-main/README.md:31-47

---

### 6.2 Distributed Lock Prevention ⬜
**Priority**: Low (assumes single instance deployment)
**Description**: Prevent multiple instances from executing same action

**Options**:
1. File-based lock (~/.qube-manager/lock)
2. Coordination via Nostr (publish "executing" event)
3. External lock service (Redis, etcd)

**Tasks**: TBD

---

### 6.3 Metrics & Monitoring ⬜
**Priority**: Low
**Description**: Expose metrics for monitoring

**Tasks**:
- [ ] Track: votes received, quorum checks, actions executed
- [ ] Expose Prometheus metrics endpoint
- [ ] Add health check endpoint
- [ ] Log structured JSON for parsing

---

## Progress Tracking

### Completion Status

| Phase | Progress | Completed | Total | Status |
|-------|----------|-----------|-------|--------|
| Phase 1: Event Format | 100% | 5 | 5 | ✅ Completed |
| Phase 2: Daemon Architecture | 75% | 3 | 4 | ✅ Completed* |
| Phase 3: Single Message Model | 100% | 2 | 2 | ✅ Completed |
| Phase 4: Security & Config | 75% | 3 | 4 | ✅ Completed** |
| Phase 5: Testing | 33% | 1 | 3 | 🟡 In Progress*** |
| **Overall** | **78%** | **14** | **18** | **🟡 In Progress** |

*Phase 2: Vote persistence (task 2.4) deferred to future iteration
**Phase 4: Validation failure handling (task 4.4) deferred - requires actual execution logic
***Phase 5: Documentation complete (5.3), testing tasks (5.1, 5.2) remain for future

### Legend
- ⬜ Not Started
- 🟡 In Progress
- ✅ Completed
- ⚠️ Blocked
- ❌ Failed/Skipped

---

## Recent Updates

### 2025-11-16 - Phase 5 Documentation Complete ✅
- **Completed Phase 5.3: Documentation Updates**
  - Updated README.md with:
    - New features (daemon mode, single active message, network isolation, kind 33321/3333)
    - Updated config.yaml example showing network and node_id fields
    - Updated usage flow (10 steps reflecting daemon architecture)
    - Updated send-message examples with --hash and --network flags
    - New Event Format section documenting kind=33321 and kind=3333 structures
    - Rewrote "How It Works" section explaining daemon behavior and vote superseding
  - Updated CLAUDE.md with:
    - New daemon architecture workflow (8 steps)
    - Updated Core Workflow explaining long-running daemon components
    - Updated CandidateAction and Voting System descriptions
    - Added Single Active Message Model section
    - Updated Event Format section with tag-based examples
    - Updated all line number references
    - Added validation.go to File Organization
    - Updated configuration details (network, node_id)
  - Updated ROADMAP.md to reflect completion status
- **Status**: Phase 5 documentation complete. Testing tasks (5.1, 5.2) deferred for future.

### 2025-11-16 - Phase 4 Complete ✅
- **Completed Phase 4: Security & Configuration (3/4 tasks)**
  - Added Network and NodeID fields to Config struct
  - Created generateNodeID() function for UUID generation
  - Updated default config to include network="hqz" and random node_id
  - Auto-upgrade existing configs with missing network/node_id fields
  - Implemented network filtering in event processing loop
  - Updated status event publishing to use real config.Network and config.NodeID
  - Created validation.go with verifyBinaryHash() function for SHA256 verification
  - Code compiles successfully
- **Deferred**: Validation failure handling (task 4.4) - requires actual execution logic
- **Status**: Ready for Phase 5 (Testing & Integration)

### 2025-11-16 - Phase 3 Complete ✅
- **Completed Phase 3: Single Active Message Model**
  - Added `latestSignal` map to track newest event timestamp per dev pubkey
  - Added `signalActionMap` to track which action each dev's latest signal created
  - Implemented vote clearing when newer signal arrives from same dev
  - Ignore events with older timestamps from same dev
  - Verified highest version selection already works correctly (from Phase 1)
  - Code compiles successfully
- **Status**: Ready for Phase 4 (Security & Configuration)

### 2025-11-16 - Phase 2 Mostly Complete 🟡
- **Completed Phase 2: Long-Running Daemon Architecture (3/4 tasks)**
  - Removed 10-second timeout, added graceful shutdown with SIGINT/SIGTERM handling
  - Implemented `checkAndExecuteQuorum()` function for periodic quorum checks
  - Added 60-second ticker for automatic quorum checking
  - Restructured main loop as long-running daemon with goroutines per relay
  - Added thread-safe mutex locking for actions/votes maps
  - Converted relay connections to parallel goroutines with WaitGroup
  - Added context cancellation checks in event processing loops
  - Removed old single-run quorum check code
  - Code compiles successfully
- **Deferred**: Vote persistence (2.4) - will implement in future iteration
- **Status**: Ready for Phase 3 (Single Active Message Model)

### 2025-11-16 - Phase 1 Complete ✅
- **Completed Phase 1: Event Format Migration**
  - Added tag helper functions (`getTagValue`, `hasTag`)
  - Updated event subscription from kind=1 to kind=33321 with `d` tag filter
  - Rewrote event processing loop to parse tags instead of JSON
  - Updated `send-message` command to send kind=33321 events with required tags
  - Implemented kind=3333 status reports for action acknowledgements
  - Updated `CandidateAction` struct with `Hash`, `Network`, `OriginalPubkey` fields
  - Code compiles successfully

### 2025-11-16 - Initial Setup
- Created initial roadmap
- Identified 5 critical compatibility issues
- Defined 5 implementation phases with 18 major tasks
- Documented all required changes with file locations and code examples

---

## Notes & Decisions

### Why Not Just Update Qubestr?
Qubestr's tag-based approach is correct for Nostr best practices. Tags are queryable, standardized, and enable efficient filtering. JSON content is opaque to relays. Changing qubestr would defeat its purpose as a specialized, validated relay.

### Why Long-Running Daemon?
The 10-second timeout makes real quorum coordination impossible. Votes arriving after timeout are lost. A daemon can accumulate votes over hours/days until quorum is reached, matching real-world dev coordination patterns.

### Why Single Active Message?
Kind 33321 is "addressable/replaceable" by design. Newer events from the same dev supersede older ones. This prevents confusion from multiple competing versions and matches qubestr's relay behavior.

### Hash Validation Implementation
Will validate hash BEFORE executing upgrade. On mismatch, will publish kind=3333 failure event and add to history (to prevent retry loop). This requires access to the binary before execution - may need download step.

---

## Questions / Blockers

- **Q**: Where do binaries get downloaded from? Need URL source.
- **Q**: Should vote persistence use separate votes.yaml or extend history.yaml?
- **Q**: Should we implement NIP-42 auth in Phase 1 or defer to Phase 6?
- **Q**: What should happen if network is not configured? Default to "hqz"?
- **Q**: How to generate node_id? Random UUID or derive from pubkey?

---

## References

- [Qubestr README](qubestr-main/README.md)
- [HyperQube Events Specification](qubestr-main/hyperqube-events.md)
- [Qubestr Validation Logic](qubestr-main/internal/handlers/validation.go)
- [NIP-01: Basic Protocol](https://github.com/nostr-protocol/nips/blob/master/01.md)
- [NIP-42: Authentication](https://github.com/nostr-protocol/nips/blob/master/42.md)
