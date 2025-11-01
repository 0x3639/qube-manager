# qube-manager

A decentralized quorum-based manager for coordinating upgrades and reboots across distributed systems using the Nostr protocol.

## Overview

Qube-manager listens to Nostr events from a configured list of trusted public keys (follows) and performs actions when a quorum threshold is reached. It uses semantic versioning to select the latest version when multiple actions meet the quorum requirement.

## Features

- **Quorum-based decision making**: Actions require agreement from a configurable number of trusted parties
- **Semantic versioning**: Automatically selects the highest version that meets quorum
- **Idempotent operations**: Tracks action history to prevent duplicate executions
- **Two action types**:
  - `upgrade`: Trigger version upgrades
  - `reboot`: Trigger reboots with a new genesis URL
- **Nostr integration**: Uses the decentralized Nostr protocol for communication
- **Key management**: Automatically generates and stores Nostr keypairs
- **Message publishing**: Send upgrade/reboot proposals to the network

## Installation

### Prerequisites

- Go 1.19 or higher

### Build

```bash
go build -o qube-manager .
```

## Configuration

Qube-manager stores its configuration in `~/.qube-manager/` (or a custom directory specified with `--config-dir`).

### Config Files

**`config.yaml`**: Main configuration
```yaml
relays:
  - wss://nostr.zenon.network
follows:
  - npub1sr47j9awvw2xa0m4w770dr2rl7ylzq4xt9k5rel3h4h58sc3mjysx6pj64
quorum: 1
```

- `relays`: List of Nostr relay WebSocket URLs to connect to
- `follows`: List of npub (Nostr public keys) to trust for voting
- `quorum`: Minimum number of votes required to trigger an action

**`keys.json`**: Your Nostr identity (auto-generated)
```json
{
  "nsec": "nsec1...",
  "npub": "npub1..."
}
```

**`history.yaml`**: Tracks completed actions to prevent re-execution

## Usage

### Basic Operation

Run the manager to listen for and process messages:

```bash
./qube-manager
```

The manager will:
1. Connect to configured relays
2. Subscribe to events from followed npubs
3. Parse upgrade/reboot messages
4. Count votes for each action
5. Execute the highest version action that meets quorum
6. Publish a "done" event upon completion
7. Save the action to history to prevent duplicate execution

### Command-Line Options

```bash
./qube-manager [options] [command]
```

**Options:**
- `--dry-run`: Perform a trial run without saving actions or publishing events
- `--config-dir <path>`: Use a custom configuration directory (default: `~/.qube-manager`)
- `--verbose`: Enable verbose logging including go-nostr debug logs

**Commands:**

#### send-message

Publish an upgrade or reboot proposal to the network:

```bash
./qube-manager send-message -type <upgrade|reboot> -version <semver> [options]
```

**Flags:**
- `-type`: Message type: `upgrade` or `reboot` (required)
- `-version`: Semantic version (e.g., `v1.2.3`) (required)
- `-genesis`: Genesis URL (required for `reboot` type)
- `-extra`: Additional metadata (optional)
- `-dry-run`: Print message instead of sending

**Examples:**

```bash
# Propose an upgrade to v1.5.0
./qube-manager send-message -type upgrade -version v1.5.0

# Propose a reboot with new genesis
./qube-manager send-message -type reboot -version v2.0.0 -genesis https://example.com/genesis.json

# Dry run to preview the message
./qube-manager send-message -type upgrade -version v1.5.0 -dry-run
```

### Display Your Keys

View your Nostr public and private keys:

```bash
cat ~/.qube-manager/keys.json
```

Or with pretty formatting:

```bash
cat ~/.qube-manager/keys.json | python3 -m json.tool
```

## Message Format

### Upgrade Message

```json
{
  "type": "upgrade",
  "version": "v1.5.0",
  "extraData": "optional metadata"
}
```

### Reboot Message

```json
{
  "type": "reboot",
  "version": "v2.0.0",
  "genesis": "https://example.com/genesis.json",
  "extraData": "optional metadata"
}
```

## How It Works

1. **Listening**: The manager connects to configured Nostr relays and subscribes to kind=1 (text note) events from trusted npubs

2. **Voting**: Each message from a followed npub counts as one vote for that specific action (version + type + genesis)

3. **Quorum**: When an action receives votes from at least `quorum` different npubs, it becomes eligible

4. **Selection**: Among all eligible actions not in history, the one with the highest semantic version is selected

5. **Execution**: The selected action is logged and a "done" event is published back to the network

6. **History**: The action is saved to history to ensure it won't be executed again

## Logging

Logs are written to both:
- Standard output
- `~/.qube-manager/qube-manager.log`

## Security Considerations

- **Private Key**: Your `nsec` (private key) in `keys.json` should be kept secure. Anyone with access can sign messages as you.
- **File Permissions**: Keys are stored with 0600 permissions (owner read/write only)
- **Trusted Follows**: Only add npubs to the `follows` list that you trust to propose upgrades/reboots
- **Quorum Setting**: Set the quorum high enough to prevent a single compromised key from triggering actions

## Development

### Project Structure

```
qube-manager/
├── main.go         # Entry point and main logic
├── config.go       # Configuration loading and validation
├── keys.go         # Nostr keypair management
├── messages.go     # Message types and send-message command
├── history.go      # Action history tracking
└── logging.go      # Logging configuration
```

### Dependencies

- `github.com/nbd-wtf/go-nostr` - Nostr protocol implementation
- `github.com/Masterminds/semver/v3` - Semantic versioning
- `gopkg.in/yaml.v3` - YAML parsing

## License

See LICENSE file for details.
