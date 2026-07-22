# distkv

A distributed key-value store built in Go as a learning project for concurrency and distributed systems concepts.

## Status

Early development. Currently implements a single-node, in-memory key-value store with concurrent-safe access.

## Features

- **Get** — retrieve a value by key (returns value + existence bool)
- **Set** — store a key-value pair
- **Delete** — remove a key
- **Concurrent-safe** — uses `sync.RWMutex` (shared locks for reads, exclusive locks for writes)

## Usage

```go
import "distkv/store"

s := store.NewStore()
s.Set("color", "blue")

val, ok := s.Get("color")
if ok {
    fmt.Println("got:", val)
}

s.Delete("color")
```

## Run tests

```bash
go run -race ./cmd/distkv/
```
