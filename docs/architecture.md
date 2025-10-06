## Architecture: End-to-End Flow

This document walks through the full request path: accept → parse (RESP) → dispatch → mutate state → durability (AOF) → respond, and outlines how we evolve toward Raft-based replication.

### Components
- `main.go`: TCP accept loop, per-connection handler
- `resp/reader.go`: Parses RESP requests into `resp.Value`
- `resp/writer.go`: Marshals `resp.Value` responses
- `commandhandler/handlers.go`: Command registry and ping
- `commandhandler/commands.go`: In-memory state for `SET/GET`, `HSET/HGET`
- `aof/aof.go`: Append-only durability with checksums and rewrite
- `Raft/node.go`: Propose/Apply interface (single-node immediate-commit stub)

### Request → Response Flow (Today)
1. `net.Listen(":6379")` in `main.go`; each accepted connection is handled by `handleConnection`.
2. For each connection:
   - `resp.NewResp(conn).Read()` reads a RESP Array: `[Bulk("SET"), Bulk("key"), Bulk("val")]`.
   - The first bulk element is the command name. Remaining are arguments.
3. Command dispatch:
   - Lookup in `commandhandler.Handlers`.
   - For mutating commands (`SET`, `HSET`): append to AOF first via `aof.Write(value)`.
   - Execute handler to update in-memory structures.
4. Response creation:
   - Handlers return a `resp.Value` (e.g., `string: OK`, `bulk: value`, or `null`).
   - `conn.Write(result.Marshal())` writes the RESP-formatted response back to the client.

### Why RESP?
RESP is the simple, widely adopted protocol used by Redis. Benefits:
- Efficient to parse and generate
- Supports strings, arrays, integers, errors, and nulls
- Enables easy interoperability with tooling like `redis-cli`

See `docs/resp.md` for full details and examples.

### Durability & Recovery
- On startup, `aof.Read(fn)` replays valid entries to rebuild in-memory state.
- During request processing, mutating commands are first recorded via `aof.Write(value)`.
  - The write is framed: start marker → length → payload → checksum → end marker.
  - `fsync` policy controls durability latency trade-offs.

See `docs/persistence.md` for AOF format, checksums, fsync policies, and rewrite.

### Moving Toward Distributed (Raft)
We introduce a `raft.Node` interface:
- `Propose(ctx, cmd)` to submit a command for replication
- `ApplyCh()` to deliver committed entries for application

Incremental migration plan:
1. Replace direct AOF+apply in `handleConnection` with `raft.Propose` for mutating commands.
2. Consume from `ApplyCh()` in a single goroutine that:
   - Parses the command (RESP payload)
   - Appends to AOF
   - Applies to in-memory state
3. Swap `NewSingleNode` with a real Raft node implementation and add transport.

This preserves correctness: state changes only occur on committed entries.


