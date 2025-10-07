## Raft Integration Overview

This project integrates Raft to provide ordered, replicated writes and disk-backed consensus, while keeping the existing AOF persistence for committed state.

### Components
- `Raft/Config.go`: Raft `Config`, `Role`, and `LogEntry` types
- `Raft/Storage.go`: `Storage` interface and `HardState`
- `Raft/FileStorage.go`: Disk-backed Raft storage (`hardstate.json`, `log.bin`)
- `Raft/MemoryStorage.go`: In-memory storage for testing
- `Raft/NodeImpl.go`: Minimal Raft `Node` implementation with immediate local commit (no networking yet)
- `main.go`: Wires Raft under `USE_RAFT=1` and applies committed commands to AOF and memory

### Data Flow
1. Client sends a write (e.g., SET)
2. Server proposes to Raft via `Node.Propose`
3. Raft appends to Raft log (persisted to disk by `FileStorage`)
4. Once committed (single-node immediate; in-memory demo uses quorum):
   - Parses the RESP payload
   - Appends to AOF
   - Applies to in-memory state
5. Server responds to the client

### Disk Artifacts
- `raftdata/hardstate.json`: JSON `{current_term, voted_for}`
- `raftdata/log.bin`: Binary sequence of entries: `term:uint64, index:uint64, len:uint64, data:bytes`
- `database.aof`: Existing append-only file for committed state machine updates

### Configuration
Enable Raft with environment variables:
- `USE_RAFT=1` to enable Raft
- `RAFT_DIR=./raftdata` (optional) to set storage directory

With Raft disabled, the server behaves like a single-node using only AOF for durability.

### Multi-node Demo (in one process)
- `go test ./tests -run TestRaftClusterDemo -v`
- Starts 3 nodes with `MemoryTransport`, elects a leader, and ensures a proposal is accepted by the leader.

### Future Work
- AppendEntries/RequestVote RPCs and leader election (added: in-memory transport, basic election/heartbeat)
- Log replication across peers (basic quorum path demoed in-process)
- Snapshots and log compaction
- Cluster membership changes


