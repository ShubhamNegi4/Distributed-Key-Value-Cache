package raft

// memoryStorage is an in-memory implementation of Storage for volatile clusters.
// Not durable across restarts; intended for pure in-memory deployments.
type memoryStorage struct {
	storageBase
	hs  HardState
	log []LogEntry
}

// NewMemoryStorage returns a Storage that keeps all state in memory.
func NewMemoryStorage() Storage {
	return &memoryStorage{
		hs:  HardState{},
		log: nil,
	}
}

func (ms *memoryStorage) LoadHardState() (hs HardState, lastIndex uint64, err error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	if len(ms.log) == 0 {
		return ms.hs, 0, nil
	}
	return ms.hs, ms.log[len(ms.log)-1].Index, nil
}

func (ms *memoryStorage) StoreHardState(hs HardState) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.hs = hs
	return nil
}

func (ms *memoryStorage) Append(entries []LogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.log = append(ms.log, entries...)
	return nil
}

func (ms *memoryStorage) Entries(from, to uint64) ([]LogEntry, error) {
	if to < from {
		return nil, nil
	}
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	if len(ms.log) == 0 {
		return nil, nil
	}
	// log is not guaranteed to be 1-indexed in slice offset; scan filter
	out := make([]LogEntry, 0, to-from)
	for i := 0; i < len(ms.log); i++ {
		idx := ms.log[i].Index
		if idx >= from && idx < to {
			out = append(out, ms.log[i])
		}
	}
	return out, nil
}

func (ms *memoryStorage) TruncateSuffix(index uint64) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	// Keep entries with Index <= index
	keep := ms.log[:0]
	for _, e := range ms.log {
		if e.Index <= index {
			keep = append(keep, e)
		} else {
			break
		}
	}
	ms.log = keep
	return nil
}
