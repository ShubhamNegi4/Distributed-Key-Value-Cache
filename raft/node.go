package raft

import (
	"context"
	"sync"
)

// Command represents a replicated command payload (RESP-encoded bytes recommended)
type Command []byte

// ApplyMsg is delivered to the state machine when a command commits.
type ApplyMsg struct {
	Data Command
}

// Node is the interface our server will depend on. Start simple with single-node behavior.
type Node interface {
	// Propose submits a command for replication. It should return once the
	// command is committed (single node: immediate) or ctx is canceled.
	Propose(ctx context.Context, cmd Command) error

	// ApplyCh returns a read-only channel of committed entries for the state machine.
	ApplyCh() <-chan ApplyMsg

	// Stop gracefully shuts down the node.
	Stop()
}

// singleNode is a trivial in-memory implementation that immediately commits.
type singleNode struct {
	applyCh chan ApplyMsg
	stopCh  chan struct{}
	once    sync.Once
}

// NewSingleNode returns a Node that commits proposals immediately.
func NewSingleNode(buffer int) Node {
	return &singleNode{
		applyCh: make(chan ApplyMsg, buffer),
		stopCh:  make(chan struct{}),
	}
}

func (n *singleNode) Propose(ctx context.Context, cmd Command) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-n.stopCh:
		return context.Canceled
	case n.applyCh <- ApplyMsg{Data: cmd}:
		return nil
	}
}

func (n *singleNode) ApplyCh() <-chan ApplyMsg {
	return n.applyCh
}

func (n *singleNode) Stop() {
	n.once.Do(func() {
		close(n.stopCh)
		close(n.applyCh)
	})
}
