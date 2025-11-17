# qube-manager

A decentralized quorum-based manager for coordinating upgrades and reboots across distributed systems using the Nostr protocol.

## Overview

Qube-manager listens to Nostr events from a configured list of trusted public keys (follows) and performs actions when a quorum threshold is reached. It uses semantic versioning to select the latest version when multiple actions meet the quorum requirement.

## Features

- **Quorum-based decision making**: Actions require agreement from a configurable number of trusted parties
- **Semantic versioning**: Automatically selects the highest version that meets quorum
- **Idempotent operations**: Tracks action history to prevent duplicate executions
- **Long-running daemon**: Continuously monitors for signals and checks quorum every 60 seconds
- **Single active message model**: Newer signals from same developer supersede older ones
- **Network isolation**: Only processes events for configured network (e.g., mainnet vs testnet)
- **Two action types**:
  - `upgrade`: Trigger version upgrades with binary hash validation
  - `reboot`: Trigger reboots with a new genesis URL
- **Nostr integration**: Uses HyperSignal (kind 33321) and QubeManager (kind 3333) events
- **NIP-42 authentication**: HC1 developers authenticate when publishing upgrade signals
- **Qubestr compatibility**: Fully compatible with Qubestr relay tag-based validation
- **Key management**: Automatically generates and stores Nostr keypairs
- **Message publishing**: Send upgrade/reboot proposals to the network
- **Binary hash verification**: Infrastructure for SHA256 validation before upgrades

## Installation

### Quick Install (Recommended)

Install qube-manager with a single command (similar to Docker's installation):

**Using curl:**
```bash
curl -fsSL https://raw.githubusercontent.com/hypercore-one/qube-manager/master/scripts/install.sh | sh
```

**Using wget:**
```bash
wget -qO- https://raw.githubusercontent.com/hypercore-one/qube-manager/master/scripts/install.sh | sh
```

The installation script will:
- Detect your OS and architecture automatically
- Download the latest release binary
- Verify the SHA256 checksum
- Install to `/usr/local/bin/qube-manager`
- Create a systemd service (Linux) or launchd service (macOS)
- Generate default configuration with HC1 dev npubs pre-configured
- Set up the `~/.qube-manager` directory

**After installation:**

Linux (systemd):
```bash
# Start the service
sudo systemctl start qube-manager

# Enable on boot
sudo systemctl enable qube-manager

# Check status
sudo systemctl status qube-manager

# View logs
sudo journalctl -u qube-manager -f
```

macOS (launchd):
```bash
# Load the service
launchctl load ~/Library/LaunchAgents/com.hypercore.qube-manager.plist

# Check if running
launchctl list | grep qube-manager

# View logs
tail -f ~/.qube-manager/qube-manager.log
```

### Manual Installation

Download pre-built binaries from the [releases page](https://github.com/hypercore-one/qube-manager/releases).

### Building from Source

#### Prerequisites

- Go 1.21 or higher
- Make (optional, for using Makefile)

#### Build

**Quick build:**
```bash
go build -o qube-manager .
```

**Build with version information (recommended):**
```bash
make build
```

**Build for all platforms:**
```bash
make build-all
```

The Makefile automatically injects version information from git tags and commit hashes into the binary.

**Check version:**
```bash
./qube-manager --version
```

## Configuration

Qube-manager stores its configuration in `~/.qube-manager/` (or a custom directory specified with `--config-dir`).

### Config Files

**`config.yaml`**: Main configuration
```yaml
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
node_id: node-a1b2c3d4-e5f6-7890-abcd-ef1234567890
```

- `relays`: List of Nostr relay WebSocket URLs to connect to (you can add or remove relays as needed)
- `follows`: List of npub (Nostr public keys) to trust for voting (pre-configured with all 6 HC1 developers, fully user-editable)
- `quorum`: Minimum number of votes required to trigger an action (default: 3 out of 6 for production safety, adjust based on your security requirements)
- `network`: Network identifier (e.g., "hqz", "testnet") - only process events for this network
- `node_id`: Unique identifier for this node (auto-generated on first run)

**Default Configuration:** On first run, qube-manager creates `config.yaml` from a template pre-configured with:
- Official Qubestr relay URLs (qubestr.zenon.info and qubestr.zenon.red)
- All 6 HC1 developer npubs (George, Vilkris, Cryptofish, Deeznnutz, Coinselor, Sl0th)
- Quorum of 3 out of 6 for production safety

**You can freely edit this file** to add/remove relays, change trusted developers, or adjust the quorum threshold. The configuration is stored in plain YAML and is fully user-controllable.

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
1. Connect to configured relays (in parallel)
2. Subscribe to kind=33321 HyperSignal events from followed npubs (no authentication required)
3. Filter events by network tag (only process our network)
4. Parse upgrade/reboot messages from event tags
5. Track votes for each action (with vote clearing for superseded signals)
6. Check quorum every 60 seconds automatically
7. Execute the highest version action that meets quorum
8. Publish a kind=3333 QubeManager status event upon completion (no authentication required)
9. Save the action to history to prevent duplicate execution
10. Continue running until SIGINT/SIGTERM (Ctrl+C)

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
- `-type`: Action type: `upgrade` or `reboot` (required)
- `-version`: Semantic version (e.g., `v1.2.3`) (required)
- `-hash`: SHA256 hash of binary (required)
- `-network`: Network identifier (required, e.g., `hqz`, `testnet`)
- `-genesis`: Genesis URL (required for `reboot` type)
- `-required-by`: Unix timestamp deadline (optional for `reboot` type)
- `-dry-run`: Print event instead of sending

**Examples:**

```bash
# Propose an upgrade to v1.5.0
./qube-manager send-message -type upgrade -version v1.5.0 \
  -hash a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2 \
  -network hqz

# Propose a reboot with new genesis
./qube-manager send-message -type reboot -version v2.0.0 \
  -hash a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2 \
  -network hqz \
  -genesis https://example.com/genesis.json \
  -required-by 1704067200

# Dry run to preview the event
./qube-manager send-message -type upgrade -version v1.5.0 \
  -hash a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2 \
  -network hqz \
  -dry-run
```

### Operational Modes

Qube-manager operates in two distinct modes:

#### Pillar Mode (Daemon - Default)
**Usage**: `./qube-manager` (no arguments)

- Runs continuously as a service/daemon
- Subscribes to upgrade/reboot signals from HC1 developers
- **No authentication required** - reads kind 33321 events unauthenticated
- Performs quorum-based voting with other pillars
- Executes upgrades/reboots when quorum threshold met
- **No authentication required** - publishes kind 3333 status reports unauthenticated
- Typical users: Pillar operators, node managers

#### HC1 Developer Mode (send-message)
**Usage**: `./qube-manager send-message ...`

- One-time command execution (exits after sending)
- Publishes upgrade/reboot proposals to the network
- **Requires NIP-42 authentication** - must use authorized developer key
- Key must be in Qubestr relay's AUTHORIZED_PUBKEYS whitelist
- Typical users: HC1 core developers (6 trusted npubs)

**Authentication Summary**:
- Pillars: No authentication needed (read/write)
- HC1 Devs: NIP-42 authentication required (write only)

### Display Your Keys

View your Nostr public and private keys:

```bash
cat ~/.qube-manager/keys.json
```

Or with pretty formatting:

```bash
cat ~/.qube-manager/keys.json | python3 -m json.tool
```

## Event Format

### HyperSignal Event (Kind 33321)

Published by developers to signal an upgrade or reboot:

```json
{
  "kind": 33321,
  "tags": [
    ["d", "hyperqube"],
    ["version", "v1.5.0"],
    ["hash", "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"],
    ["network", "hqz"],
    ["action", "upgrade"]
  ],
  "content": "[hypersignal] A HyperQube upgrade has been released for network hqz..."
}
```

For reboot actions, additional tags:
- `["genesis_url", "https://example.com/genesis.json"]`
- `["required_by", "1704067200"]` (optional)

### QubeManager Event (Kind 3333)

Published by nodes to acknowledge completion:

```json
{
  "kind": 3333,
  "tags": [
    ["a", "33321:dev_pubkey:hyperqube"],
    ["p", "dev_pubkey"],
    ["version", "v1.5.0"],
    ["network", "hqz"],
    ["action", "upgrade"],
    ["status", "success"],
    ["node_id", "node-a1b2c3d4-e5f6-7890-abcd-ef1234567890"],
    ["action_at", "1703984400"]
  ],
  "content": "[qube-manager] The upgrade to version v1.5.0 has been successful..."
}
```

## How It Works

1. **Daemon Mode**: The manager runs continuously as a daemon, connecting to all configured relays in parallel

2. **Event Listening**: Subscribes to kind=33321 (HyperSignal) events from trusted npubs without authentication (Qubestr allows unauthenticated reads)

3. **Network Filtering**: Only processes events where the `network` tag matches the configured network

4. **Voting with Superseding**: Each HyperSignal from a followed npub counts as one vote. Newer signals from the same npub automatically supersede (clear votes for) their older signals

5. **Periodic Quorum Check**: Every 60 seconds, checks if any action has reached quorum

6. **Selection**: Among all eligible actions not in history, selects the one with the highest semantic version

7. **Execution**: Logs the selected action and publishes a kind=3333 status event back to the network (no authentication required)

8. **History**: Saves the action to history to ensure it won't be executed again

9. **Graceful Shutdown**: Continues running until SIGINT/SIGTERM, then cleanly shuts down all goroutines

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

### Creating Releases

To create a new release:

1. **Ensure you're on master with latest changes:**
   ```bash
   git checkout master
   git pull origin master
   ```

2. **Run the release script:**
   ```bash
   ./scripts/release.sh v1.0.0
   ```

   The script will:
   - Validate the version format (must be vX.Y.Z)
   - Check for uncommitted changes
   - Show a changelog of commits since the last release
   - Create and push a git tag

3. **GitHub Actions automatically:**
   - Builds binaries for all platforms (Linux, macOS, Windows)
   - Generates release notes from commit messages
   - Creates a GitHub release
   - Uploads binary assets with SHA256 checksums

4. **Monitor the release:**
   ```bash
   # Get your repository URL
   REPO=$(git remote get-url origin | sed 's/.*github.com[:/]\(.*\)\.git/\1/')

   # View workflow progress
   echo "https://github.com/$REPO/actions"

   # View releases
   echo "https://github.com/$REPO/releases"
   ```

**Manual release** (without script):
```bash
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

## License

See LICENSE file for details.
