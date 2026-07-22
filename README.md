# distkv

A distributed key-value store built in Go as a learning project for concurrency and distributed systems concepts.

## Status

Single-node, in-memory key-value store with HTTP API.

## Features

- **Get** — retrieve a value by key (returns value + existence bool)
- **Set** — store a key-value pair
- **Delete** — remove a key
- **Concurrent-safe** — uses `sync.RWMutex` (shared locks for reads, exclusive locks for writes)
- **HTTP API** — GET/PUT/DELETE endpoints via `/{key}` path

## Project layout

```
store/store.go    — core KV store with concurrent-safe Get/Set/Delete
server/server.go  — HTTP server wrapping the store
cmd/server/       — standalone server binary
cmd/distkv/       — test harness (starts server, runs HTTP tests)
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
