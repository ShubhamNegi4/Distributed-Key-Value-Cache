package raft

import "time"

// Role describes the node's current role in the Raft protocol.
type Role int

const (
    RoleFollower Role = iota
    RoleCandidate
    RoleLeader
)

// Config holds the runtime configuration for a Raft node.
// Peers is a list of node IDs, including this node's own ID.
// StoragePath is where Raft metadata and log are stored when using FileStorage.
type Config struct {
    ID                 string
    Peers              []string
    StoragePath        string
    ElectionTimeoutMin time.Duration
    ElectionTimeoutMax time.Duration
    HeartbeatInterval  time.Duration
}

// LogEntry represents a single entry in the Raft log.
// Index values start at 1; implementations may keep a sentinel at index 0.
type LogEntry struct {
    Term  uint64
    Index uint64
    Data  []byte
}


