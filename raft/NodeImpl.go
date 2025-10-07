package raft

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

// nodeImpl is a basic Raft node scaffold that appends proposals to the
// Raft log on disk and immediately commits them (no networking yet).
// This preserves the Node interface while we evolve towards full Raft.
type nodeImpl struct {
	mu      sync.Mutex
	cfg     Config
	storage Storage
	role    Role

	applyCh chan ApplyMsg
	quit    chan struct{}

	commitIndex uint64
	lastApplied uint64
	currentTerm uint64

	votedFor string

	// cluster
	id        string
	peers     []string
	transport Transport

	// leader state
	nextIndex  map[string]uint64
	matchIndex map[string]uint64
}

// NewNode creates a new node backed by the provided storage.
// It starts background timers to keep parity with a future full Raft node.
func NewNode(cfg Config, st Storage) Node {
	n := &nodeImpl{
		cfg:     cfg,
		storage: st,
		role:    RoleFollower,
		applyCh: make(chan ApplyMsg, 1024),
		quit:    make(chan struct{}),
	}
	n.id = cfg.ID
	n.peers = cfg.Peers
	// Initialize hard state
	if hs, lastIdx, err := st.LoadHardState(); err == nil {
		n.currentTerm = hs.CurrentTerm
		n.votedFor = hs.VotedFor
		n.commitIndex = lastIdx
		n.lastApplied = lastIdx
	}
	go n.ticker()
	return n
}

func (n *nodeImpl) Propose(ctx context.Context, cmd Command) error {
	// Append locally
	n.mu.Lock()
	term := maxU64(1, n.currentTerm)
	_, lastIdx, err := n.storage.LoadHardState()
	if err != nil {
		n.mu.Unlock()
		return err
	}
	newIdx := lastIdx + 1
	entry := LogEntry{Term: term, Index: newIdx, Data: []byte(cmd)}
	if err := n.storage.Append([]LogEntry{entry}); err != nil {
		n.mu.Unlock()
		return err
	}

	// If follower in a clustered setup, reject (not leader)
	clustered := n.transport != nil && len(n.peers) > 1
	if clustered && n.role != RoleLeader {
		n.mu.Unlock()
		return context.Canceled
	}
	// If single node or no transport, commit immediately
	single := n.transport == nil || len(n.peers) <= 1
	n.mu.Unlock()

	if single {
		n.advanceCommitTo(newIdx)
		// applied by applier in main via ApplyCh
		select {
		case <-ctx.Done():
			return ctx.Err()
		case n.applyCh <- ApplyMsg{Data: Command(entry.Data)}:
			return nil
		}
	}

	// Replicate to peers
	type ack struct{ ok bool }
	ch := make(chan ack, len(n.peers))
	majority := (len(n.peers)/2 + 1)
	// leader state snapshot
	n.mu.Lock()
	leaderId := n.id
	peers := append([]string(nil), n.peers...)
	n.mu.Unlock()

	for _, p := range peers {
		if p == leaderId {
			continue
		}
		go func(peer string) {
			ok := n.replicateToPeer(peer)
			ch <- ack{ok: ok}
		}(p)
	}

	// count acks including self
	votes := 1
	for i := 0; i < len(peers)-1; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case a := <-ch:
			if a.ok {
				votes++
			}
		}
	}
	if votes >= majority {
		n.advanceCommitTo(newIdx)
		// Notify applier
		select {
		case <-ctx.Done():
			return ctx.Err()
		case n.applyCh <- ApplyMsg{Data: Command(entry.Data)}:
			return nil
		}
	}
	return context.DeadlineExceeded
}

func (n *nodeImpl) ApplyCh() <-chan ApplyMsg {
	return n.applyCh
}

func (n *nodeImpl) Stop() {
	n.mu.Lock()
	select {
	case <-n.quit:
		// already closed
	default:
		close(n.quit)
		close(n.applyCh)
	}
	n.mu.Unlock()
}

func (n *nodeImpl) ticker() {
	rand.Seed(time.Now().UnixNano())
	for {
		timeout := n.randElectionTimeout()
		timer := time.NewTimer(timeout)
		for {
			select {
			case <-n.quit:
				timer.Stop()
				return
			case <-timer.C:
				// election timeout
				n.startElection()
				if n.getRole() == RoleLeader {
					n.runLeader()
				}
				// break inner loop to reset timeout
				goto nextCycle
			}
		}
	nextCycle:
		continue
	}
}

func maxU64(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

// WithTransport registers a transport and this node as handler.
func (n *nodeImpl) WithTransport(t Transport) *nodeImpl {
	n.mu.Lock()
	n.transport = t
	id := n.id
	n.mu.Unlock()
	if t != nil && id != "" {
		t.Register(id, n)
	}
	return n
}

func (n *nodeImpl) getRole() Role {
	n.mu.Lock()
	r := n.role
	n.mu.Unlock()
	return r
}

func (n *nodeImpl) randElectionTimeout() time.Duration {
	min := n.cfg.ElectionTimeoutMin
	max := n.cfg.ElectionTimeoutMax
	if min == 0 {
		min = 300 * time.Millisecond
	}
	if max == 0 {
		max = 600 * time.Millisecond
	}
	if max < min {
		max = min + 100*time.Millisecond
	}
	return min + time.Duration(rand.Int63n(int64(max-min+1)))
}

func (n *nodeImpl) startElection() {
	// Single-node self election if no peers/transport
	if n.transport == nil || len(n.peers) <= 1 {
		n.mu.Lock()
		n.role = RoleLeader
		n.mu.Unlock()
		return
	}
	n.mu.Lock()
	n.role = RoleCandidate
	n.currentTerm++
	term := n.currentTerm
	n.votedFor = n.id
	_ = n.storage.StoreHardState(HardState{CurrentTerm: n.currentTerm, VotedFor: n.votedFor})
	n.mu.Unlock()

	votes := 1 // self vote
	majority := (len(n.peers) / 2) + 1
	for _, p := range n.peers {
		if p == n.id {
			continue
		}
		args := RequestVoteArgs{Term: term, CandidateID: n.id}
		go func(peer string) {
			if n.transport == nil {
				return
			}
			reply, _ := n.transport.SendRequestVote(peer, args)
			if reply.VoteGranted && reply.Term == term {
				n.mu.Lock()
				if n.role == RoleCandidate {
					votes++
					if votes >= majority {
						n.role = RoleLeader
					}
				}
				n.mu.Unlock()
			} else if reply.Term > term {
				n.mu.Lock()
				if reply.Term > n.currentTerm {
					n.currentTerm = reply.Term
					n.role = RoleFollower
					n.votedFor = ""
					_ = n.storage.StoreHardState(HardState{CurrentTerm: n.currentTerm, VotedFor: n.votedFor})
				}
				n.mu.Unlock()
			}
		}(p)
	}
}

func (n *nodeImpl) runLeader() {
	hb := n.cfg.HeartbeatInterval
	if hb == 0 {
		hb = 100 * time.Millisecond
	}
	t := time.NewTicker(hb)
	defer t.Stop()
	for {
		select {
		case <-n.quit:
			return
		case <-t.C:
			if n.getRole() != RoleLeader {
				return
			}
			n.broadcastHeartbeat()
		}
	}
}

func (n *nodeImpl) broadcastHeartbeat() {
	if n.transport == nil {
		return
	}
	n.mu.Lock()
	term := n.currentTerm
	leaderCommit := n.commitIndex
	id := n.id
	peers := append([]string(nil), n.peers...)
	if n.nextIndex == nil {
		n.nextIndex = make(map[string]uint64)
	}
	if n.matchIndex == nil {
		n.matchIndex = make(map[string]uint64)
	}
	n.mu.Unlock()
	for _, p := range peers {
		if p == id {
			continue
		}
		// send empty heartbeat (no entries)
		args := AppendEntriesArgs{Term: term, LeaderID: id, LeaderCommit: leaderCommit}
		go n.transport.SendAppendEntries(p, args)
	}
}

// RPC handlers
func (n *nodeImpl) OnRequestVote(from string, args RequestVoteArgs) (RequestVoteReply, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	reply := RequestVoteReply{Term: n.currentTerm, VoteGranted: false}
	if args.Term < n.currentTerm {
		return reply, nil
	}
	if args.Term > n.currentTerm {
		n.currentTerm = args.Term
		n.role = RoleFollower
		n.votedFor = ""
	}
	if n.votedFor == "" || n.votedFor == args.CandidateID {
		n.votedFor = args.CandidateID
		_ = n.storage.StoreHardState(HardState{CurrentTerm: n.currentTerm, VotedFor: n.votedFor})
		reply.VoteGranted = true
	}
	reply.Term = n.currentTerm
	return reply, nil
}

func (n *nodeImpl) OnAppendEntries(from string, args AppendEntriesArgs) (AppendEntriesReply, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	reply := AppendEntriesReply{Term: n.currentTerm, Success: false}
	if args.Term < n.currentTerm {
		return reply, nil
	}
	if args.Term >= n.currentTerm {
		n.currentTerm = args.Term
		n.role = RoleFollower
		n.votedFor = args.LeaderID
		// Validate prevLog if provided
		if args.PrevLogIndex > 0 {
			prevTerm, ok := n.getTermAtLocked(args.PrevLogIndex)
			if !ok || prevTerm != args.PrevLogTerm {
				// reject, leader will back off
				reply.Term = n.currentTerm
				return reply, nil
			}
		}
		// Append any new entries, truncating conflicts
		if len(args.Entries) > 0 {
			// find first conflict
			for _, e := range args.Entries {
				t, ok := n.getTermAtLocked(e.Index)
				if ok && t != e.Term {
					// truncate from e.Index-1
					_ = n.storage.TruncateSuffix(e.Index - 1)
					break
				}
			}
			// append remaining entries
			// find last existing index
			// append all entries whose index is beyond current last
			_, lastIdx, _ := n.storage.LoadHardState()
			var toAppend []LogEntry
			for _, e := range args.Entries {
				if e.Index > lastIdx {
					toAppend = append(toAppend, e)
				}
			}
			if len(toAppend) > 0 {
				_ = n.storage.Append(toAppend)
			}
		}
		// Update commit index
		if args.LeaderCommit > n.commitIndex {
			n.commitIndex = args.LeaderCommit
		}
		reply.Success = true
	}
	reply.Term = n.currentTerm
	return reply, nil
}

// replicateToPeer sends entries starting at nextIndex[peer]
func (n *nodeImpl) replicateToPeer(peer string) bool {
	if n.transport == nil {
		return false
	}
	n.mu.Lock()
	if n.nextIndex == nil {
		n.nextIndex = make(map[string]uint64)
	}
	if n.matchIndex == nil {
		n.matchIndex = make(map[string]uint64)
	}
	from := n.nextIndex[peer]
	if from == 0 {
		// initialize to lastIndex+1
		_, lastIdx, _ := n.storage.LoadHardState()
		from = lastIdx + 1
	}
	// prev values
	prevIdx := from - 1
	prevTerm := uint64(0)
	if prevIdx > 0 {
		if t, ok := n.getTermAtLocked(prevIdx); ok {
			prevTerm = t
		}
	}
	// gather entries [from, last+1)
	_, lastIdx, _ := n.storage.LoadHardState()
	ents, _ := n.storage.Entries(from, lastIdx+1)
	term := n.currentTerm
	leaderId := n.id
	leaderCommit := n.commitIndex
	n.mu.Unlock()

	args := AppendEntriesArgs{Term: term, LeaderID: leaderId, PrevLogIndex: prevIdx, PrevLogTerm: prevTerm, Entries: ents, LeaderCommit: leaderCommit}
	reply, _ := n.transport.SendAppendEntries(peer, args)
	if reply.Success {
		n.mu.Lock()
		// advance next/match
		n.nextIndex[peer] = lastIdx + 1
		n.matchIndex[peer] = lastIdx
		n.mu.Unlock()
		return true
	}
	// back off nextIndex and retry once
	n.mu.Lock()
	cur := n.nextIndex[peer]
	if cur > 1 {
		n.nextIndex[peer] = cur - 1
	} else {
		n.nextIndex[peer] = 1
	}
	n.mu.Unlock()
	return false
}

func (n *nodeImpl) getTermAtLocked(index uint64) (uint64, bool) {
	if index == 0 {
		return 0, false
	}
	ents, err := n.storage.Entries(index, index+1)
	if err != nil || len(ents) == 0 {
		return 0, false
	}
	return ents[0].Term, true
}

func (n *nodeImpl) advanceCommitTo(idx uint64) {
	n.mu.Lock()
	if idx > n.commitIndex {
		n.commitIndex = idx
	}
	n.mu.Unlock()
}
