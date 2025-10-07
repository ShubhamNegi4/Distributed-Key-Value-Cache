## Distributed_Cache

A lightweight Redis-like cache server with RESP protocol support, AOF durability, and Raft-backed replication primitives. Single-node Raft is wired end-to-end; an in-memory multi-node demo runs inside one process for learning.

### Quickstart (single node)

```bash
USE_RAFT=1 RAFT_DIR=./raftdata go run .
# Server listens on 6379 with Raft hard-state/log persisted under ./raftdata

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
- Raft persistence and primitives:
  - Disk-backed Raft hard state (`currentTerm`, `votedFor`) and log (`log.bin`)
  - Minimal elections and heartbeats (in-memory transport)
  - Basic log replication and quorum commit (in-process demo)

### Project Structure
- `main.go`: TCP server and request loop
- `resp/`: RESP reader and writer
- `commandhandler/`: in-memory state and command handlers
- `aof/`: append-only file (AOF) with checksums and rewrite
- `config/`: sample AOF configurations
- `Raft/`: Raft types, storage, node implementation, and in-memory transport
- `docs/`: deep dives and design notes
- `tests/`: AOF tests and in-memory Raft demo

### Documentation
- See `docs/architecture.md` for end-to-end flow from request accept to response write
- See `docs/resp.md` for RESP framing details and examples
- See `docs/persistence.md` for AOF format, checksums, fsync policies, and rewrite
- See `docs/raft.md` for Raft integration, storage, and demo details

### Current Limitations
- Networked multi-node transport is not yet implemented; demo uses in-memory transport inside one process
- Minimal command set

### How persistence works (Raft + AOF)
- Writes: client → Raft propose → Raft log persisted → commit → apply to memory → append to AOF → respond
- Startup: load Raft hard state/log (consensus) and replay AOF to rebuild in-memory data

### Multi-node demo (in one process)
- Run: `go test ./tests -run TestRaftClusterDemo -v`
- Behavior: 3 in-memory nodes elect a leader; propose succeeds on the leader

### Roadmap
- Network transport (HTTP/TCP) for cross-process Raft replication and failover
- Snapshotting and log compaction integration with AOF
- Cluster membership and configuration changes


