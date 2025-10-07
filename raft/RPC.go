package raft

// RequestVote RPC
type RequestVoteArgs struct {
	Term         uint64
	CandidateID  string
	LastLogIndex uint64
	LastLogTerm  uint64
}

type RequestVoteReply struct {
	Term        uint64
	VoteGranted bool
}

// AppendEntries RPC (also heartbeat)
type AppendEntriesArgs struct {
	Term         uint64
	LeaderID     string
	PrevLogIndex uint64
	PrevLogTerm  uint64
	Entries      []LogEntry
	LeaderCommit uint64
}

type AppendEntriesReply struct {
	Term    uint64
	Success bool
}

// RaftRPCHandler is implemented by a Raft node to handle inbound RPCs.
type RaftRPCHandler interface {
	OnRequestVote(from string, args RequestVoteArgs) (RequestVoteReply, error)
	OnAppendEntries(from string, args AppendEntriesArgs) (AppendEntriesReply, error)
}

// Transport abstracts sending Raft RPCs to peers.
type Transport interface {
	Register(id string, h RaftRPCHandler)
	SendRequestVote(to string, args RequestVoteArgs) (RequestVoteReply, error)
	SendAppendEntries(to string, args AppendEntriesArgs) (AppendEntriesReply, error)
}
