## Persistence: AOF, Checksums, Fsync, Rewrite

This server persists mutating commands to an Append-Only File (AOF). On startup, the AOF is replayed to rebuild in-memory state.

### Write Path
When a mutating command (e.g., SET, HSET) arrives, the server writes a framed record:
1. Random 8-byte start marker
2. 8-byte little-endian payload length
3. RESP-encoded payload bytes
4. 8-byte CRC64 checksum of payload
5. The same 8-byte end marker (must match start marker)

This framing allows detection of partial writes and corruption.

### Fsync Policies
- Always: `fsync` on every write (safest, slowest)
- Every Second: background ticker `fsync` once per second
- No: rely on OS to flush (fastest, least durable on crash)

Configured via `aof.AofConfig.FsyncPolicy`.

### Checksums
- We compute CRC64 (ISO polynomial) over the payload and store it with the record.
- On read, we compute again and compare; mismatch triggers truncation at the last known good boundary.

### Recovery
`Aof.Read(fn)` scans the file in order, verifying:
- Start marker read OK
- Length matches available bytes
- Checksum matches
- End marker equals start marker

If any step fails (EOF or mismatch), the file is truncated at the current offset, preventing propagation of corruption.

### Rewrite/Compaction
- Background monitor can trigger rewrite based on file size and age thresholds.
- Rewrite copies only valid payloads into a fresh file, compacting history.
- After rewrite, the old file is atomically replaced.

### Configuration
See `config/config.go` for convenience builders (durability, performance, balanced, dev).


