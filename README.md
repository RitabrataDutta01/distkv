# distkv

A distributed key-value store built in Go as a learning project for concurrency and distributed systems concepts.

## Status

Phase 3a — raw TCP leader–replica replication. A leader node accepts client requests and forwards mutations to replicas. Data survives restart via JSON snapshot per node.

## Features

- **Get / Set / Delete** — core KV operations
- **Raw TCP wire protocol** — `PUT|/key|val`, `GET|/key`, `DELETE|/key`
- **Leader–replica replication** — leader forwards PUT/DELETE to all configured peers
- **Crash-safe persistence** — atomic temp-file + rename writes with fsync
- **Concurrent-safe** — `sync.RWMutex` (shared reads, exclusive writes)
- **Config-driven roles** — JSON config determines primary vs replica behavior

## Project layout

```
store/store.go      — core KV store with concurrent-safe Get/Set/Delete
store/db.go         — snapshot save/load (JSON, atomic write, fsync)
server/helpers.go   — Config, Snap, LoadConfig, ForwardToPeer, ForwardToAllPeers
server/server.go    — leader TCP server (RunServer, HandleLeaderConnection)
server/nodes.go     — replica TCP server (RunNode, HandleNodeConnection)
server/config-*.json — per-node config files
cmd/server/         — server binary; reads config path from CLI arg
cmd/distkv/         — test harness (starts server, runs TCP tests)
snapshot/           — per-node on-disk snapshots (gitignored)
```

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

## Run tests

```bash
go run -race ./cmd/distkv/
```
