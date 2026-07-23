# distkv

A distributed key-value store built in Go as a learning project for concurrency and distributed systems concepts.

## Status

Single-node, persistent key-value store with HTTP API. Data survives restart via JSON snapshot to disk.

## Features

- **Get** — retrieve a value by key (returns value + existence bool)
- **Set** — store a key-value pair
- **Delete** — remove a key
- **Persistence** — JSON snapshot file written atomically on every mutation, loaded on startup
- **Crash-safe** — writes to a temp file then renames atomically; corrupt snapshots never overwrite valid data
- **Concurrent-safe** — uses `sync.RWMutex` (shared locks for reads, exclusive locks for writes)
- **HTTP API** — GET/PUT/DELETE endpoints via `/{key}` path

## Project layout

```
store/store.go    — core KV store with concurrent-safe Get/Set/Delete
store/db.go       — snapshot save/load (JSON, atomic write, crash-safe)
server/server.go  — HTTP server wrapping the store
cmd/server/       — standalone server binary
cmd/distkv/       — test harness (starts server, runs HTTP tests)
snapshot/         — on-disk snapshot directory
```

## Usage (standalone server)

```bash
go run ./cmd/server/
```

Then use curl:

```bash
curl -X PUT -d 'blue' http://localhost:8080/color
curl http://localhost:8080/color
curl -X DELETE http://localhost:8080/color
```

## Run tests

```bash
go run -race ./cmd/distkv/
```
