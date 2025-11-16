# Qubestr Relay

A specialized Nostr relay for the HyperQube Network.

## Overview

Qubestr is a custom Nostr relay built using the [Khatru](https://github.com/fiatjaf/khatru) framework. It is designed to be a secure and reliable communication channel for the HyperQube network, a decentralized infrastructure management system.

The relay exclusively handles two custom event kinds:
- **HyperSignal (Kind 33321)**: Used by HC1 developers to broadcast commands to HyperQube nodes, such as software upgrades or network reboots.
- **QubeManager (Kind 3333)**: Used by HyperQube nodes to report back the status (`success` or `failure`) of a command they have executed.

All interactions with this relay require NIP-42 authentication to ensure a secure and spam-resistant environment.

For a detailed technical specification of these custom Nostr events, please see [hyperqube-events.md](./hyperqube-events.md).

## Features

### Core Features
- **Built with Khatru**: A high-performance, lightweight, and customizable relay framework.
- **PostgreSQL Backend**: Uses a PostgreSQL database for robust and persistent event storage.
- **Mandatory Authentication**: All connections must authenticate via NIP-42. Both publishing and reading events are protected.
- **Dockerized**: Comes with `docker-compose.yml` for easy and reproducible deployment.
- **Specialized Logic**: Contains strict validation rules tailored specifically for HyperQube network events.

### Custom Event Support
- **HyperSignal (Kind 33321)**: Events for broadcasting node management commands.
- **QubeManager (Kind 3333)**: Events for nodes to report the status of executed commands.

### Content & Security Policies

#### 1. **Strict Event Validation**
- **HyperSignal (Kind 33321)**:
    - Must be published by an authenticated and whitelisted pubkey from a HC1 developer (`AUTHORIZED_PUBKEYS`).
    - Requires a `d` tag with the value `hyperqube`.
    - Requires `version`, `hash`, `network`, and `action` tags.
    - `action` must be either `upgrade` or `reboot`.
    - If `action` is `reboot`, the `genesis_url` and `required_by` tags are also required.
- **QubeManager (Kind 3333)**:
    - Requires an `a` tag referencing the kind 33321 `HyperSignal` event it is responding to.
    - Requires `version`, `network`, `action`, `status`, `node_id`, and `action_at` tags.
    - `status` must be either `success` or `failure`.
    - If `status` is `failure`, an `error` tag with a reason is required.

#### 2. **Authentication**
- **NIP-42 Required**: All clients must authenticate to read or write any events. The relay will send an `AUTH` challenge on connect.
- **Permissioned Writing**: Only public keys defined in the `AUTHORIZED_PUBKEYS` environment variable can publish `HyperSignal` (kind 33321) events. This ensures that only trusted administrators can issue commands to the network.

## API Endpoints
- **WebSocket**: `ws://localhost:3334`
- **HTTP**: `http://localhost:3334` (Returns a simple HTML welcome page)

## Quick Start

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/coinselor/qubestr.git
    cd qubestr
    ```

2.  **Configure your environment:**
    Copy the example `.env.example` file to `.env` and fill in the required values.
    ```bash
    cp .env.example .env
    ```
    At a minimum, you must set `AUTHORIZED_PUBKEYS` and `RELAY_ADMIN_PUBKEY`.

3.  **Run with Docker Compose:**
    This is the recommended method for production.
    ```bash
    docker-compose up --build -d
    ```
    The relay will be available at `ws://localhost:3334`.

## Development

### Prerequisites
- Go (version 1.21 or higher)
- Docker (for running PostgreSQL)
- An understanding of the Nostr protocol.

### Development Setup

1.  **Clone and enter the directory:**
    ```bash
    git clone https://github.com/coinselor/qubestr.git
    cd qubestr
    ```

2.  **Install dependencies:**
    ```bash
    go mod tidy
    ```

3.  **Start a PostgreSQL database:**
    You can use the provided `docker-compose.yml` to start only the database:
    ```bash
    docker-compose up -d postgres
    ```
    Alternatively, run PostgreSQL locally.

4.  **Set up environment variables:**
    Create a `.env` file and configure it for your local setup. The default database connection values in the `.env.example` are configured for the Docker Compose setup.

5.  **Run the relay:**
    ```bash
    go run ./cmd/qubestr
    ```

### Project Structure
```
qubestr/
├── cmd/
│   └── qubestr/
│       └── main.go       # Main application entry point & server setup
├── internal/
│   └── handlers/
│       └── validation.go   # All event validation logic
├── .env.example        # Example environment variables
├── go.mod              # Go module definition
├── Dockerfile          # Container definition
└── docker-compose.yml  # Docker composition for relay and database
```

### Configuration
The relay is configured via environment variables loaded from a `.env` file.

| Variable | Description | Default |
| :--- | :--- | :--- |
| `DB_USER` | PostgreSQL username. | `postgres` |
| `DB_PASSWORD` | PostgreSQL password. | `postgres` |
| `DB_HOST` | PostgreSQL host. | `localhost` |
| `DB_PORT` | PostgreSQL port. | `5432` |
| `DB_NAME` | PostgreSQL database name. | `qubestr` |
| `DB_SSLMODE` | PostgreSQL SSL mode. | `disable` |
| `RELAY_ADMIN_PUBKEY` | The Nostr pubkey of the relay administrator (for NIP-11). | `""` |
| `AUTHORIZED_PUBKEYS` | A comma-separated list of pubkeys authorized to publish kind 33321 events. | `""` |
| `PORT` | The port for the relay to listen on. | `3334` |
| `HOST` | The host interface for the relay to bind to. | `0.0.0.0` |
| `DB_QUERY_LIMIT`| The default query limit for event requests. | `100` |
| `DB_KEEP_RECENT_EVENTS`| Whether to keep an in-memory cache of recent events. | `false` |

## API Examples

The examples below provide a quick overview. For the complete specification, including all required tags and validation rules, please refer to [hyperqube-events.md](./hyperqube-events.md).

### Publish a `HyperSignal` Event (Kind 33321)
This event signals to nodes on the `hqz` network to upgrade to version `0.0.9`.

#### Upgrade Action
```json
{  
  "id": "32-bytes lowercase hex-encoded sha256 of serialized event data",  
  "pubkey": "32-bytes lowercase hex-encoded public key of dev",  
  "created_at": 1703980800,  
  "kind": 33321,  
  "tags": [  
    ["d", "hyperqube"],  
    ["version", "v0.0.9"],
    ["hash", "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"],    
    ["network", "hqz"],  
    ["action", "upgrade"]
  ],  
  "content": "[hypersignal] A HyperQube upgrade has been released. Please update binary to version v0.0.9.",  
  "sig": "64-bytes lowercase hex signature"  
}
```

#### Reboot Action
```json
{  
  "id": "32-bytes lowercase hex-encoded sha256 of serialized event data",  
  "pubkey": "32-bytes lowercase hex-encoded public key of hc1 dev",  
  "created_at": 1703980800,  
  "kind": 33321,  
  "tags": [  
    ["d", "hyperqube"],  
    ["version", "v0.0.9"],
    ["hash", "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"],
    ["network", "hqz"],  
    ["action", "reboot"],  
    ["genesis_url", "https://example.com/path/to/genesis.json"],  
    ["required_by", "1704067200"]  
  ],  
  "content": "[hypersignal] A HyperQube reboot for version v0.0.9 has been scheduled. Please reboot by 2024-01-01T00:00:00Z.",  
  "sig": "64-bytes lowercase hex signature"  
}
```

### Publish a `QubeManager` Event (Kind 3333)
This event reports a successful `upgrade` action from a specific node.

```json
{  
  "id": "32-bytes lowercase hex-encoded sha256 of serialized event data",   
  "pubkey": "32-bytes lowercase hex-encoded public key of qube-managed node",  
  "created_at": 1703984400,  
  "kind": 3333,  
  "tags": [  
    ["a", "33321:hc1-dev-pubkey-hex:hyperqube", "wss://relay1.hypercore.one"],  
    ["p", "hc1-dev-pubkey-hex", "wss://relay1.hypercore.one"],  
    ["version", "v0.0.9"],  
    ["network", "hqz"],  
    ["action", "upgrade"],    
    ["status", "success"],
    ["node_id", "node-unique-identifier"],    
    ["action_at", "1703984400"]  
  ],  
  "content": "[qube-manager] The upgrade to version <version> has been successful.",  
  "sig": "64-bytes lowercase hex signature"  
}
```

## Contributing

Contributions are welcome! Please follow these steps:
1. Fork the repository.
2. Create your feature branch (`git checkout -b feature/your-feature`).
3. Commit your changes (`git commit -m 'Add some feature'`).
4. Push to the branch (`git push origin feature/your-feature`).
5. Open a Pull Request.

## License

This project is licensed under the MIT License.

## Acknowledgments

- Built with [Khatru](https://github.com/fiatjaf/khatru)
- Uses [go-nostr](https://github.com/nbd-wtf/go-nostr) for Nostr protocol implementation
- PostgreSQL storage via [eventstore](https://github.com/fiatjaf/eventstore)
