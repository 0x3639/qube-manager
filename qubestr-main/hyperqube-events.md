# HyperQube Nostr Event Definitions

## Basics of Nostr Events

Before diving into the custom events, it's helpful to understand the basic structure of any Nostr event. A Nostr event is simply a JSON object with a few standard fields. This structure is the foundation for all communication on the network.

| Field        | Description                                                                                                                              |
|--------------|------------------------------------------------------------------------------------------------------------------------------------------|
| `id`         | A unique identifier for the event. It's the SHA256 hash of the event's data, ensuring each event is unique and verifiable.                |
| `pubkey`     | The public key of the user or service that created the event. This is the identity of the event's author.                                  |
| `created_at` | A Unix timestamp (in seconds) marking when the event was created.                                                                        |
| `kind`       | A number that specifies the type of event. Kinds 0-9999 are for regular events, with higher ranges reserved for special types of events. |
| `tags`       | An array of arrays that add extra, queryable metadata to an event. Tags are used to link events, reference users, or add other structured data. |
| `content`    | The main content of the event, such as a text message or, in our case, a structured description.                                         |
| `sig`        | A cryptographic signature that proves the `pubkey` owner actually created this event.                                                    |

---

## Kind 33321: HyperSignal Event (Action Directive)

### Overview

HyperSignal Events (Kind 33321) are used by HyperCore One developers to broadcast directives, such as software upgrades or scheduled reboots, to HyperQube nodes. These are **addressable events**, a special type of replaceable event defined in the Nostr protocol.

The term "address" is used because the `d` tag helps create a unique, queryable coordinate for the event (specifically, `<kind>:<pubkey>:<d-tag>`), much like how a URL points to a specific webpage. This allows relays to keep and serve only the latest version of a directive from a specific developer (`pubkey`) for that particular "address."

For these events, the address identifier is always "hyperqube". So, for any given developer, a relay will only store the single most recent HyperSignal event for "hyperqube".

### Specification

| Property        | Value                                  |
|-----------------|----------------------------------------|
| **Kind Number** | 33321                                  |
| **Event Type**  | Addressable (Parameterized Replaceable) |
| **Identifier**  | `d` tag with value "hyperqube"         |

### Content Format

The `content` field is a human-readable string summarizing the action being announced.

#### Schema

```json
"content": "A string describing the action, e.g., '[hypersignal] A HyperQube upgrade has been released. Please update binary to version v0.0.9.'"
```

### Tags

| Tag Name      | Description                                                                 | Format                                      | Required          | Example                               |
|---------------|-----------------------------------------------------------------------------|---------------------------------------------|-------------------|---------------------------------------|
| `d`           | **The identifier that makes this event "addressable."** While "identifier" is a good description, the Nostr protocol uses the term "address" to convey that this tag creates a unique, retrievable location for the latest version of an event. The value is static ("hyperqube") to create a well-known address for nodes to query. | `["d", "hyperqube"]`                        | Yes               | `["d", "hyperqube"]`                  |
| `version`     | The target version for the action (e.g., software version).                 | `["version", "<version_string>"]`           | Yes               | `["version", "v0.0.9"]`               |
| `hash`        | SHA256 hash of the binary or artifact.                                      | `["hash", "<sha256_hex_string>"]`           | Yes               | `["hash", "a1b2c3d4..."]`             |
| `network`     | Identifier for the target network (e.g., "hqz").                           | `["network", "<network_id>"]`               | Yes               | `["network", "hqz"]`                  |
| `action`      | The type of action to be performed: "upgrade" or "reboot".                  | `["action", "<action_type>"]`               | Yes               | `["action", "upgrade"]`               |
| `genesis_url` | URL for genesis data, required for "reboot" actions.                        | `["genesis_url", "<url_string>"]`           | For "reboot"      | `["genesis_url", "https://example.com/genesis.json"]` |
| `required_by` | Unix timestamp indicating a deadline for a "reboot" action.                 | `["required_by", "<unix_timestamp_string>"]`| For "reboot"      | `["required_by", "1704067200"]`       |

### Client Behavior (HyperQube Nodes / `qube-manager`)

1.  Subscribe to Kind 33321 events from a list of trusted developer public keys, filtering for the `["d", "hyperqube"]` tag.
2.  Upon receiving an event, parse its tags to determine the `action`, `version`, `hash`, `genesis_url` (for reboots), and any deadlines.
3.  Nodes should prioritize the event with the latest `created_at` timestamp as the current valid directive.
4.  Before applying an "upgrade", nodes should verify the `hash` of the downloaded artifact if possible.
5.  After successfully performing or attempting the directed action, the node should emit a "Node Event" (Kind 3333) as an acknowledgement.

### Relay Behavior

Relays should treat Kind 33321 events as parameterized replaceable events. For each `pubkey`, they should only store and serve the latest event that has a `["d", "hyperqube"]` tag.

### Use Cases

-   Announcing mandatory or recommended software updates for HyperQube nodes.
-   Coordinating scheduled reboots across the HyperQube network.
-   Distributing critical configuration changes or parameters.

### Example: Upgrade Action

```json
{
  "id": "<32-bytes lowercase hex-encoded sha256 of serialized event data>",
  "pubkey": "<32-bytes lowercase hex-encoded public key of dev>",
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
  "sig": "<64-bytes lowercase hex signature>"
}
```

### Example: Reboot Action

```json
{
  "id": "<32-bytes lowercase hex-encoded sha256 of serialized event data>",
  "pubkey": "<32-bytes lowercase hex-encoded public key of dev>",
  "created_at": 1703980800,
  "kind": 33321,
  "tags": [
    ["d", "hyperqube"],
    ["version", "v0.0.9"],
    ["network", "hqz"],
    ["action", "reboot"],
    ["genesis_url", "https://example.com/path/to/genesis.json"],
    ["required_by", "1704067200"]
  ],
  "content": "[hypersignal] A HyperQube reboot for version v0.0.9 has been scheduled. Please reboot by 2024-01-01T00:00:00Z.",
  "sig": "<64-bytes lowercase hex signature>"
}
```

## Kind 3333: Qube Manager Event (Action Acknowledgement)

### Overview

Qube Manager Events (Kind 3333) are emitted by individual nodes (e.g., via `qube-manager`) to acknowledge the status or outcome of an action directive they received (typically a HyperSignal Event, Kind 33321). These are **regular events**, meaning relays preserve the history of all acknowledgements submitted by nodes. This creates a permanent, auditable trail of a node's actions.

### Specification

| Property        | Value   |
|-----------------|---------|
| **Kind Number** | 3333    |
| **Event Type**  | Regular |

### Content Format

The `content` field is a human-readable string summarizing the acknowledgement.

#### Schema

```json
"content": "A string describing the outcome, e.g., '[qube-manager] The upgrade to version v0.0.9 has been successful.'"
```

### Tags

| Tag Name    | Description                                                                    | Format                                                                 | Required | Example                                                                 |
|-------------|--------------------------------------------------------------------------------|------------------------------------------------------------------------|----------|-------------------------------------------------------------------------|
| `a`         | **Address tag pointing to the event being acknowledged.** It references the Kind 33321 HyperSignal Event using the format `<kind>:<pubkey>:<d-tag-value>`. This creates a direct, queryable link back to the specific directive. | `["a", "33321:<dev_pubkey_hex>:hyperqube", "<optional_relay_url>"]`    | Yes      | `["a", "33321:abcdef123...:hyperqube", "wss://relay.example.com"]`    |
| `p`         | **Pubkey tag referencing the author of the original directive.** This tags the developer who issued the Kind 33321 event, allowing them to be easily notified and to find all acknowledgements for their directives.  | `["p", "<dev_pubkey_hex>", "<optional_relay_url>"]`                    | Rec.     | `["p", "abcdef123...", "wss://relay.example.com"]`                     |
| `version`   | The version of the software involved in the acknowledged action.          | `["version", "<version_string>"]`                                      | Yes      | `["version", "v0.0.9"]`                                                 |
| `network`   | Identifier for the HyperQube Network the node is part of.                      | `["network", "<network_id>"]`                                          | Yes      | `["network", "hqz"]`                                                    |
| `action`    | The type of action that was performed by the node ("upgrade" or "reboot").     | `["action", "<action_type>"]`                                          | Yes      | `["action", "upgrade"]`                                                 |
| `status`    | The outcome or current status of the action on the node.                       | `["status", "<status_string>"]` (e.g., "success", "failure") | Yes      | `["status", "success"]`                                                 |
| `node_id`   | A unique identifier for the reporting HyperQube node.                          | `["node_id", "<unique_node_identifier>"]`                              | Yes      | `["node_id", "hyperqube-node-xyz789"]`                                  |
| `action_at` | Unix timestamp indicating when the action was completed or its status recorded.| `["action_at", "<unix_timestamp_string>"]`                             | Yes      | `["action_at", "1703984400"]`                                           |
| `error`     | A brief error message if the `status` is "failure".                            | `["error", "<error_message_string>"]`                                  | Optional | `["error", "hash mismatch"]` `["error", "URL not found"]`                                            |


### Client Behavior (Monitoring Tools)

1.  Subscribe to Kind 3333 events.
2.  Can filter events based on the `a` tag to track acknowledgements for a specific HyperSignal Event.
3.  Can filter events based on the `p` tag to see all acknowledgements related to directives issued by a specific developer.
4.  Use the `node_id` tag to differentiate acknowledgements from various nodes.
5.  Monitor the `status`, `action_at`, and `error` tags to track the fleet's compliance and health.

### Relay Behavior

Relays should treat Kind 3333 events as regular events. All events of this kind should be stored, preserving the history of node acknowledgements.

### Use Cases

-   Tracking the rollout progress of software upgrades across nodes.
-   Confirming that nodes have successfully rebooted after a directive.
-   Monitoring the operational status and compliance of individual nodes in the fleet.
-   Auditing the history of actions performed by nodes.

### Example

```json
{
  "id": "<32-bytes lowercase hex-encoded sha256 of serialized event data>",
  "pubkey": "<32-bytes lowercase hex-encoded public key of node>",
  "created_at": 1703984400,
  "kind": 3333,
  "tags": [
    ["a", "33321:<dev_pubkey_hex>:hyperqube", "wss://relay1.hypercore.one"],
    ["p", "<dev_pubkey_hex>", "wss://relay1.hypercore.one"],
    ["version", "v0.0.9"],
    ["network", "hqz"],
    ["action", "upgrade"],
    ["status", "success"],
    ["node_id", "node-unique-identifier-123"],
    ["action_at", "1703984300"]
  ],
  "content": "[qube-manager] The upgrade to version v0.0.9 has been successful on node-unique-identifier-123.",
  "sig": "<64-bytes lowercase hex signature>"
}
```
