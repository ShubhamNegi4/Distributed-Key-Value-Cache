## Distributed_Cache

A lightweight Redis-like cache server with RESP protocol support, AOF durability, and a path to Raft-based replication. Currently supports a robust single-node with append-only file (AOF) persistence and checksum-protected log format.

### Quickstart

```bash
go run .
# Server listens on 6379

# In another terminal
redis-cli -p 6379 PING
redis-cli -p 6379 SET foo bar
redis-cli -p 6379 GET foo
```

### Features
- RESP request/response handling (compatible with common `redis-cli` commands)
- In-memory key-value and hash storage: `SET`, `GET`, `HSET`, `HGET`, `PING`
- Append-Only File (AOF) persistence with:
  - Framed entries (marker, length, payload, checksum, marker)
  - CRC64 checksums for integrity
  - fsync policies: always, every second, or OS-managed
  - Background rewrite/compaction (configurable)
- Raft scaffolding for future distributed consensus

### Project Structure
- `main.go`: TCP server and request loop
- `resp/`: RESP reader and writer
- `commandhandler/`: in-memory state and command handlers
- `aof/`: append-only log with checksums and rewrite
- `config/`: sample AOF configurations
- `Raft/`: Raft node interface and single-node stub
- `docs/`: deep dives and design notes

### Documentation
- See `docs/architecture.md` for end-to-end flow from request accept to response write
- See `docs/resp.md` for RESP framing details and examples
- See `docs/persistence.md` for AOF format, checksums, fsync policies, and rewrite

### Current Limitations
- Single-node semantics for writes; Raft integration planned
- Minimal command set

### Roadmap (High-level)
1) Integrate Raft `Propose`/`ApplyCh` in the command pipeline
2) Implement multi-node transport and basic leader election
3) Snapshotting and log compaction integration with AOF
4) Cluster membership and configuration changes


