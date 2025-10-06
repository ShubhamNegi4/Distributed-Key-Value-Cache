## RESP Protocol Usage

We implement a subset of RESP as used by Redis, enabling interoperability with `redis-cli`.

### Types Implemented
- Simple Strings: `+OK\r\n`
- Bulk Strings: `$3\r\nfoo\r\n`
- Arrays: `*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$3\r\nval\r\n`
- Errors: `-ERR message\r\n`
- Null Bulk: `$-1\r\n`

### Reader (`resp/reader.go`)
- Reads a leading type byte and dispatches to `readArray` or `readBulk`.
- Arrays are recursive; each element is parsed via `Read()`.
- Bulk strings read length, then payload, then trailing CRLF.

### Writer (`resp/writer.go`)
- `Value.Marshal()` serializes values into RESP wire format for client responses.

### Examples
Request:
```
*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n
```
Response:
```
+OK\r\n
```

Null GET:
```
*2\r\n$3\r\nGET\r\n$7\r\nunknown\r\n
```
```
$-1\r\n
```

### Why RESP here?
- Simplicity: easy to implement and debug.
- Compatibility: `redis-cli` works out of the box.
- Framing: binary-safe bulk strings and arrays with explicit lengths.


