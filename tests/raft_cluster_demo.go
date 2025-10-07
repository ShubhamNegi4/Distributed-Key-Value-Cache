package tests

import (
	r "Distributed_Cache/Raft"
	"context"
	"testing"
	"time"
)

func TestRaftClusterDemo(t *testing.T) {
	// Build in-memory transport and three nodes
	tr := r.NewMemoryTransport()

	cfgA := r.Config{ID: "A", Peers: []string{"A", "B", "C"}, HeartbeatInterval: 100 * time.Millisecond}
	cfgB := r.Config{ID: "B", Peers: []string{"A", "B", "C"}, HeartbeatInterval: 100 * time.Millisecond}
	cfgC := r.Config{ID: "C", Peers: []string{"A", "B", "C"}, HeartbeatInterval: 100 * time.Millisecond}

	stA := r.NewMemoryStorage()
	stB := r.NewMemoryStorage()
	stC := r.NewMemoryStorage()

	a := r.NewNode(cfgA, stA)
	b := r.NewNode(cfgB, stB)
	c := r.NewNode(cfgC, stC)

	r.AttachTransport(a, tr)
	r.AttachTransport(b, tr)
	r.AttachTransport(c, tr)

	// For simplicity of demo, we just try proposing through IDs and see who accepts
	nodes := []struct{ id string }{{"A"}, {"B"}, {"C"}}
	var leader string
	// give time for election
	time.Sleep(800 * time.Millisecond)
	for _, cand := range nodes {
		var n r.Node
		switch cand.id {
		case "A":
			n = a
		case "B":
			n = b
		case "C":
			n = c
		}
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		err := n.Propose(ctx, r.Command([]byte("demo")))
		cancel()
		if err == nil {
			leader = cand.id
			break
		}
	}
	if leader == "" {
		t.Fatalf("no leader elected")
	}
}
