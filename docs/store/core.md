# Core store implementation

This document covers the in-memory store logic in store/store.go.

## Data flow

```mermaid
flowchart TD
    A[Set/Delete/Get] --> B[Store mutex]
    B --> C[In-memory map]
    C --> D[WAL append]
    D --> E[Periodic flush]
    C --> F[Snapshot compaction]
```

## Store structure

### Store

The Store struct owns the actual database state for a node. It contains:

- the key/value map
- a mutex for concurrency control
- path information for persistence files
- version tracking
- last saved timestamp
- the WAL file handle
- a channel for async WAL writes
- shutdown coordination via WaitGroup

This makes the store both thread-safe and durable enough to survive restarts.

## Construction and startup

### NewStore

NewStore creates the parent directory for persistence files, opens the WAL file, initializes the in-memory map, and loads persisted state from disk.

It also starts two background workers:

- walFlusher
- StartCompaction

This is the lifecycle for an active store instance.

## WAL flushing

### walFlusher

This goroutine writes queued WAL records as they arrive and calls flushLog on a one-second ticker. Set and Delete enqueue records but return before the background writer necessarily writes or syncs them, so an abrupt failure can lose recent acknowledged mutations.

### flushLog

This asks the operating system to sync the WAL file with disk. It is called by the background logger, not synchronously by each Set or Delete.

## Compaction

### StartCompaction

This goroutine runs every five minutes. It writes a snapshot, then truncates the WAL so the log does not grow forever. WAL appends are handled by a separate goroutine, so this is not a transactional snapshot-and-log boundary.

This is how the project keeps storage bounded while preserving a recoverable history.

## Core operations

### Get

Returns the value for a key and a boolean indicating whether the key exists.

### Set

Adds or updates a key/value pair, increments the version, and appends a WAL record.

### Delete

Removes a key, increments the version, and appends a delete record to the WAL.

### Snapshot

Serializes the current state to JSON for leader/follower sync. This is used when the leader sends state to a follower.

### ApplySnapshot

Replaces the local in-memory map with the incoming remote snapshot and writes it to disk. This is how a follower catches up after it has fallen behind.

## Shutdown

### Close

Close prevents new writes, closes the log channel, waits for the WAL writer to flush, and closes the WAL file. The server entrypoints do not currently wire process shutdown to Store.Close.

## Why the design works

The combination of in-memory state, frequent log syncing, and periodic snapshot compaction gives the project a simple but practical durable key/value model. It is not a full database engine, but it is a clear pattern for crash recovery and follower synchronization.
