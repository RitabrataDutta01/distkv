# DistKV

<div align="center">

![Go](https://img.shields.io/badge/Go-1.26.5%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)
![Protocol](https://img.shields.io/badge/Protocol-TCP%20%2B%20WAL-6A5ACD?style=for-the-badge)
![Status](https://img.shields.io/badge/Status-Learning%20Project-FFA500?style=for-the-badge)

</div>

A lightweight distributed key-value store built in Go to explore leader/follower replication, TCP networking, versioned snapshots, and durable recovery.

## ✨ What this project does

- Stores values by key in a simple in-memory map
- Runs a leader node that accepts writes
- Queues PUT and DELETE requests for peers currently marked healthy
- Periodically checks peer health with TCP `PING` / `PONG`
- Persists state using a write-ahead log and snapshot compaction
- Recovers from disk on restart by replaying WAL data after the last snapshot

## 🏗️ Architecture

```mermaid
flowchart LR
    Client --> Leader
    Leader --> StoreA
    Leader --> Replica1
    Leader --> Replica2
    Replica1 --> StoreB
    Replica2 --> StoreC
```

The leader is the intended source of truth for writes. Replication is best-effort: the leader queues mutations for peers without waiting for their acknowledgment, and a full peer queue can drop a mutation. Followers request a full snapshot from the leader at startup and every five seconds, which is the mechanism for reconciling missed updates. This is not consensus or strongly consistent replication. The current address handling is intended for local nodes: server code prepends `127.0.0.1` to configured peer and leader addresses.

## 📚 Documentation

The project is organized by code area:

- [docs/cluster-overview.md](docs/cluster-overview.md) — documentation index
- [docs/server/leader.md](docs/server/leader.md) — leader lifecycle and request flow
- [docs/server/follower.md](docs/server/follower.md) — follower startup and sync flow
- [docs/server/helpers.md](docs/server/helpers.md) — config, protocol, and peer forwarding helpers
- [docs/store/core.md](docs/store/core.md) — main store implementation
- [docs/store/persistence.md](docs/store/persistence.md) — WAL and snapshot persistence

## 🗂️ Repository layout

```text
.
├── cmd/
│   └── server/            # CLI entrypoints for server processes
├── docs/                  # grouped project documentation
├── server/
│   ├── server.go          # leader logic and health checks
│   ├── nodes.go           # follower sync and handling
│   ├── helpers.go         # config + request protocol helpers
│   └── config-*.json     # per-node config files
├── store/
│   ├── store.go           # key/value store logic
│   └── db.go              # snapshot + WAL persistence helpers
├── snapshot/              # current snapshot files
├── Dockerfile             # container build definition
├── Makefile               # common task shortcuts
├── distkv.yaml            # draft Kubernetes manifest; not compatible with current runtime as-is
├── go.mod                 # Go module definition
├── README.md              # this file
└── .gitignore             # repo ignores
```

## 🚀 Quick start

Start the leader and replicas in separate terminals:

```bash
go run ./cmd/server/ server/config-primary.json
go run ./cmd/server/ server/config-replica1.json
go run ./cmd/server/ server/config-replica2.json
```

Then interact with the leader over TCP:

```bash
printf "PUT|/color|blue\n" | nc localhost 8081
printf "GET|/color\n"     | nc localhost 8081
printf "DELETE|/color\n"  | nc localhost 8081
```

Check a replica:

```bash
printf "GET|/color\n" | nc localhost 8082
```

## ⚙️ Configuration

Each node reads a JSON config file.

```json
{
  "role": "replica",
  "address": ":8082",
  "peers": [":8081"],
  "leader": ":8081"
}
```

Fields:

- `role` — use exactly `primary` for the leader; any other value starts a follower
- `address` — TCP listen address for the node
- `peers` — peer ports the leader will forward writes to (for example, `:8082`)
- `leader` — leader port a follower uses for sync checks (for example, `:8081`)

The checked-in `distkv.yaml` is a draft, not a supported deployment configuration: it uses role names the runtime does not treat as primary and hostnames that the current loopback-based address construction cannot connect to. The JSON files under `server/` are the working examples for the current implementation.

## 🔄 Sync and persistence model

### Snapshot format

Each node stores a JSON snapshot with:

- `version` — current mutation count
- `timestamp` — last snapshot time in nanoseconds
- `data` — key/value map

Example:

```json
{"version":2,"timestamp":1785735782610875224,"data":{"/color":"blue","/animal":"dog"}}
```

### Replica sync flow

On startup and every five seconds, a follower sends `SYNC|<version>` to the leader:

1. The leader compares the follower version to its own.
2. If versions match, it replies with `SYNC_OK`.
3. If versions differ, it replies with `SYNC_DATA|<snapshot JSON>`.
4. The follower applies the full snapshot. This reconciles data, but there is no incremental log transfer.

### WAL recovery

The store appends mutations to a buffered WAL and syncs that file in the background on a one-second interval. On restart, it loads the snapshot and replays every WAL record still present. WAL records do not carry versions, so replay changes the data without advancing the snapshot's version; recent mutations are also not guaranteed to have reached disk when an operation returns. Applying a leader snapshot does not clear a follower's WAL, so old local records may be replayed over the snapshot after a restart. Treat persistence as educational, not as a guarantee against abrupt process or machine failure.

## 🧵 Wire protocol

Requests are newline-terminated and pipe-delimited. Special characters are escaped so they survive transport and storage cleanly.

| Raw value | On the wire |
|-----------|-------------|
| `|` | `\|` |
| `\` | `\\` |
| newline | `\n` |

Example frames:

- `PUT|/color|blue\n`
- `GET|/color\n`
- `DELETE|/color\n`
- `SYNC|42\n`

## 🧪 Testing

To run the Go test suite:

```bash
go test ./...
```

## ✅ Notes

This project is intentionally small and educational: it focuses on core distributed systems ideas such as replication, persistence, versioning, and recovery rather than production-grade consensus, strong durability guarantees, or sharding.

