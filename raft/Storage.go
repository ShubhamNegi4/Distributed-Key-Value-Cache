package raft

import "sync"

// HardState represents the term and vote persisted by Raft.
type HardState struct {
    CurrentTerm uint64 `json:"current_term"`
    VotedFor    string `json:"voted_for"`
}

// Storage abstracts durable Raft state management. Distinct from AOF.
type Storage interface {
    LoadHardState() (hs HardState, lastIndex uint64, err error)
    StoreHardState(hs HardState) error
    Append(entries []LogEntry) error
    Entries(from, to uint64) ([]LogEntry, error)
    TruncateSuffix(index uint64) error
}

// storageBase provides a mutex for concrete storage implementations.
type storageBase struct {
    mu sync.RWMutex
}


