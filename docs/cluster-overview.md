# DistKV documentation index

This folder is grouped by code area so each part of the system has its own explanation instead of a single monolithic document.

## Server-side code

- [server/leader.md](server/leader.md) — leader lifecycle, health checks, and request handling
- [server/follower.md](server/follower.md) — follower startup, sync loop, and local request flow
- [server/helpers.md](server/helpers.md) — config, transport encoding, and peer forwarding helpers

## Store-side code

- [store/core.md](store/core.md) — main in-memory store behavior and lifecycle
- [store/persistence.md](store/persistence.md) — WAL and snapshot recovery model

## High-level view

The project is a small distributed key/value system with:

- a leader that accepts writes and coordinates peer replication
- follower nodes that synchronize with the leader and serve local reads
- a durable store layer that persists state through snapshots and a WAL

Replication is best-effort rather than consensus-based: the leader queues mutations without waiting for follower acknowledgments, and followers periodically request a full snapshot. The local JSON configurations are the supported examples; `distkv.yaml` is a draft Kubernetes manifest that does not match the current runtime's role and address handling.

This grouping keeps each area easier to read without changing any code.
