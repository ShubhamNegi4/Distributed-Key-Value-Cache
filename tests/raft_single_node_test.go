package tests

import (
	raft "Distributed_Cache/raft"
	persist "Distributed_Cache/aof"
	handle "Distributed_Cache/commandhandler"
	resp "Distributed_Cache/resp"
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestSingleNodeRaftProposeApply(t *testing.T) {
	// clean state
	handle.SETs = map[string]string{}

	aof, err := persist.NewAof("test_raft_single.aof")
	if err != nil {
		t.Fatalf("failed to create aof: %v", err)
	}
	defer func() {
		aof.Close()
		os.Remove("test_raft_single.aof")
	}()

	node := raft.NewSingleNode(16)

	// start applier inline
	done := make(chan struct{})
	go func() {
		for msg := range node.ApplyCh() {
			rd := resp.NewResp(bytes.NewReader([]byte(msg.Data)))
			val, err := rd.Read()
			if err != nil {
				t.Errorf("parse error: %v", err)
				continue
			}
			cmd := strings.ToUpper(val.Array[0].Bulk)
			args := val.Array[1:]
			if cmd == "SET" {
				_ = aof.Write(val)
			}
			if h, ok := handle.Handlers[cmd]; ok {
				_ = h(args)
			}
			close(done)
			return
		}
	}()

	// propose a SET command
	cmd := resp.Value{Typ: "array", Array: []resp.Value{
		{Typ: "bulk", Bulk: "SET"},
		{Typ: "bulk", Bulk: "rk"},
		{Typ: "bulk", Bulk: "rv"},
	}}.Marshal()

	if err := node.Propose(context.Background(), raft.Command(cmd)); err != nil {
		t.Fatalf("propose failed: %v", err)
	}

	<-done

	// verify state
	handle.SETsMu.RLock()
	got, ok := handle.SETs["rk"]
	handle.SETsMu.RUnlock()
	if !ok || got != "rv" {
		t.Fatalf("expected rk=rv, got %v %v", ok, got)
	}
}
