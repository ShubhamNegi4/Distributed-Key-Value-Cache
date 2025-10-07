package raft

import "sync"

// memoryTransport is a simple in-memory Transport for multi-node
// simulations within a single process.
type memoryTransport struct {
	mu       sync.RWMutex
	handlers map[string]RaftRPCHandler
}

func NewMemoryTransport() Transport {
	return &memoryTransport{handlers: make(map[string]RaftRPCHandler)}
}

func (t *memoryTransport) Register(id string, h RaftRPCHandler) {
	t.mu.Lock()
	t.handlers[id] = h
	t.mu.Unlock()
}

func (t *memoryTransport) SendRequestVote(to string, args RequestVoteArgs) (RequestVoteReply, error) {
	t.mu.RLock()
	h := t.handlers[to]
	t.mu.RUnlock()
	if h == nil {
		return RequestVoteReply{}, nil
	}
	return h.OnRequestVote("", args)
}

func (t *memoryTransport) SendAppendEntries(to string, args AppendEntriesArgs) (AppendEntriesReply, error) {
	t.mu.RLock()
	h := t.handlers[to]
	t.mu.RUnlock()
	if h == nil {
		return AppendEntriesReply{}, nil
	}
	return h.OnAppendEntries("", args)
}
