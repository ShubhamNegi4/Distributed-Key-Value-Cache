package main

import (
	raft "Distributed_Cache/Raft"
	persist "Distributed_Cache/aof"
	handle "Distributed_Cache/commandhandler"
	resp "Distributed_Cache/resp"
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	fmt.Println("Listening on port: 6379")
	//Create a new server
	l, err := net.Listen("tcp", ":6379")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer l.Close()

	aof, err := persist.NewAof("database.aof")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer aof.Close()

	// Replay AOF file on startup
	err = aof.Read(func(value resp.Value) {
		command := strings.ToUpper(value.Array[0].Bulk)
		args := value.Array[1:]

		handler, ok := handle.Handlers[command]
		if !ok {
			fmt.Println("Invalid command: ", command)
			return
		}

		handler(args)
	})
	if err != nil {
		fmt.Printf("Error replaying AOF file: %v\n", err)
	}

	// Optional Raft integration (disk-backed) behind env toggle
	useRaft := os.Getenv("USE_RAFT") == "1"
	var node raft.Node
	if useRaft {
		// Build config and storage
		dataDir := os.Getenv("RAFT_DIR")
		if dataDir == "" {
			dataDir = filepath.Join(".", "raftdata")
		}
		st, err := raft.NewFileStorage(dataDir)
		if err != nil {
			fmt.Println("raft storage error:", err)
			return
		}
		cfg := raft.Config{ID: "node-1", Peers: []string{"node-1"}}
		node = raft.NewNode(cfg, st)
		// Start applier: consume committed entries, persist to AOF, apply to memory
		go startApplier(node, aof)
	}

	// Accept connections in a loop
	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}

		// Handle each connection in a goroutine
		go handleConnection(conn, aof, node)
	}
}

func handleConnection(conn net.Conn, aof *persist.Aof, node raft.Node) {
	defer conn.Close()

	for {
		r := resp.NewResp(conn)
		value, err := r.Read()
		if err != nil {
			fmt.Println(err)
			return
		}
		if value.Typ != "array" {
			fmt.Println("Invalid request, expected array")
			continue
		}
		if len(value.Array) == 0 {
			fmt.Println("Invalid request, expected array length > 0")
			continue
		}
		command := strings.ToUpper(value.Array[0].Bulk)
		args := value.Array[1:]

		handler, ok := handle.Handlers[command]
		if !ok {
			// write RESP error back without noisy console logs
			conn.Write(resp.Value{Typ: "error", Str: "ERR unknown command"}.Marshal())
			continue
		}
		// If Raft enabled, route mutating commands via Propose
		if node != nil && (command == "SET" || command == "HSET") {
			// Propose RESP-framed payload; apply happens in applier goroutine
			payload := value.Marshal()
			_ = node.Propose(context.Background(), raft.Command(payload))
			// Respond optimistically OK (single-node commit is immediate)
			conn.Write(resp.Value{Typ: "string", Str: "OK"}.Marshal())
			continue
		}

		// Non-mutating or Raft disabled: persist then apply for mutating commands
		if node == nil && (command == "SET" || command == "HSET") {
			aof.Write(value)
		}
		result := handler(args)
		conn.Write(result.Marshal())
	}
}

// startApplier consumes committed commands from Raft and applies them:
// 1) parse RESP payload
// 2) append to AOF
// 3) dispatch to in-memory handlers
func startApplier(node raft.Node, aof *persist.Aof) {
	for msg := range node.ApplyCh() {
		// Parse RESP payload
		rd := resp.NewResp(bytes.NewReader([]byte(msg.Data)))
		val, err := rd.Read()
		if err != nil || val.Typ != "array" || len(val.Array) == 0 {
			continue
		}
		cmd := strings.ToUpper(val.Array[0].Bulk)
		args := val.Array[1:]

		// Persist only mutating commands
		if cmd == "SET" || cmd == "HSET" {
			_ = aof.Write(val)
		}
		// Apply to in-memory state
		if h, ok := handle.Handlers[cmd]; ok {
			_ = h(args)
		}
	}
}
