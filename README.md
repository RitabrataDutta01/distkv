# distkv

A distributed key-value store built in Go as a learning project for concurrency and distributed systems concepts.

## Status

Phase 4 — versioned snapshots with replica catch-up on startup. A leader node accepts client requests and forwards mutations to replicas. Each node persists its own versioned JSON snapshot; on startup a replica compares its snapshot version against the leader's and, on mismatch, pulls the leader's full state so it comes back in sync after downtime.

## Features

- **Get / Set / Delete** — core KV operations
- **Raw TCP wire protocol** — `PUT|/key|val`, `GET|/key`, `DELETE|/key`, `SYNC|<version>`
- **Leader–replica replication** — leader forwards PUT/DELETE to all configured peers
- **Versioned snapshots** — every save stores a monotonic `version` and `timestamp` alongside the data
- **Replica startup sync** — replicas ping the leader on boot and download a full snapshot on version mismatch
- **Crash-safe persistence** — atomic temp-file + rename writes with fsync
- **Concurrent-safe** — `sync.RWMutex` (shared reads, exclusive writes)
- **Config-driven roles** — JSON config determines primary vs replica behavior

## Project layout

```
store/store.go      — core KV store: Get/Set/Delete, Version/SavedAt, Snapshot/ApplySnapshot
store/db.go         — snapshot save/load (JSON, atomic write, fsync) and writeFile helper
server/helpers.go   — Config, Snap, LoadConfig, ForwardToPeer, ForwardToAllPeers
server/server.go    — leader TCP server (RunServer, HandleLeaderConnection, SYNC handler)
server/nodes.go     — replica TCP server (RunNode, HandleNodeConnection, SyncWithLeader)
server/config-*.json — per-node config files
cmd/server/         — server binary; reads config path from CLI arg
cmd/distkv/         — test harness (starts server, runs TCP tests)
snapshot/           — per-node on-disk snapshots (gitignored)
```

## Snapshot format

Each node persists a single JSON file per run, e.g. `snapshot/8081.json`:

```json
{"version":2,"timestamp":1785735782610875224,"data":{"/color":"blue","/animal":"dog"}}
```

- `version` — monotonic counter, bumped on every successful write
- `timestamp` — unix nanoseconds of the last successful save
- `data` — the key/value map

Legacy flat-map snapshots (no `version`/`data` keys) are detected and loaded automatically as version 0.

## Replica sync flow

On startup a replica (`RunNode`) sends `SYNC|<version>` to its leader:

1. The leader compares the replica's version with its own.
2. **Equal** → replies `SYNC_OK`; nothing to do.
3. **Mismatch** → replies `SYNC_DATA|<snapshot JSON>`; the replica applies it via `ApplySnapshot`, overwriting its local state and keeping the leader's exact version/timestamp.

The leader is authoritative: any mismatch (even if the replica's version is higher) results in the replica copying the leader's state.

## Usage (cluster)

In separate terminals:

```bash
go run ./cmd/server/ server/config-primary.json
go run ./cmd/server/ server/config-replica1.json
go run ./cmd/server/ server/config-replica2.json
```

Then interact with the leader:

```bash
printf "PUT|/color|blue" | nc localhost 8081
printf "GET|/color"      | nc localhost 8081
printf "DELETE|/color"   | nc localhost 8081
```

Verify replication — same GET on a replica should return the value:

```bash
printf "GET|/color" | nc localhost 8082
```

## Configuration

Each node reads a JSON config file. Example replica config:

```json
{
  "role": "replica",
  "address": ":8082",
  "peers": [":8081"],
  "leader": ":8081"
}
```

- `role` — `primary` or `replica`; determines whether the node runs the leader or replica server
- `address` — listen address for this node
- `peers` — addresses the leader forwards mutations to (replicas list the primary here too)
- `leader` — primary address that replicas ping for startup sync (only used on replicas)

## Run tests

```bash
go run -race ./cmd/distkv/
```
