package raft

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// FileStorage is a simple file-backed Storage implementation using two files:
// - hardstate.json: JSON-encoded HardState
// - log.bin: length-prefixed entries (term:uint64, index:uint64, len:uint64, data:[]byte)
// It is intentionally simple and suitable for a prototype.
type FileStorage struct {
	mu      sync.RWMutex
	dir     string
	hsPath  string
	logPath string
}

// NewFileStorage initializes (and creates if missing) a file-backed storage at dir.
func NewFileStorage(dir string) (Storage, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	fs := &FileStorage{
		dir:     dir,
		hsPath:  filepath.Join(dir, "hardstate.json"),
		logPath: filepath.Join(dir, "log.bin"),
	}
	// Ensure files exist
	if _, err := os.Stat(fs.hsPath); errors.Is(err, os.ErrNotExist) {
		b, _ := json.Marshal(HardState{CurrentTerm: 0, VotedFor: ""})
		if err := os.WriteFile(fs.hsPath, b, 0o644); err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(fs.logPath); errors.Is(err, os.ErrNotExist) {
		f, err := os.Create(fs.logPath)
		if err != nil {
			return nil, err
		}
		if err := f.Close(); err != nil {
			return nil, err
		}
	}
	return fs, nil
}

func (fs *FileStorage) LoadHardState() (hs HardState, lastIndex uint64, err error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	b, err := os.ReadFile(fs.hsPath)
	if err != nil {
		return HardState{}, 0, err
	}
	if err := json.Unmarshal(b, &hs); err != nil {
		return HardState{}, 0, err
	}
	lastIndex, err = fs.scanLastIndex()
	if err != nil {
		return HardState{}, 0, err
	}
	return hs, lastIndex, nil
}

func (fs *FileStorage) StoreHardState(hs HardState) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	b, err := json.Marshal(hs)
	if err != nil {
		return err
	}
	tmp := fs.hsPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, fs.hsPath)
}

func (fs *FileStorage) Append(entries []LogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()

	f, err := os.OpenFile(fs.logPath, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := bufio.NewWriter(f)
	for _, e := range entries {
		if err := writeEntry(buf, e); err != nil {
			return err
		}
	}
	return buf.Flush()
}

func (fs *FileStorage) Entries(from, to uint64) ([]LogEntry, error) {
	if to < from {
		return nil, fmt.Errorf("invalid range [%d,%d)", from, to)
	}
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	f, err := os.Open(fs.logPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []LogEntry
	r := bufio.NewReader(f)
	for {
		e, err := readEntry(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if e.Index >= from && e.Index < to {
			out = append(out, e)
		}
	}
	return out, nil
}

func (fs *FileStorage) TruncateSuffix(index uint64) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	f, err := os.Open(fs.logPath)
	if err != nil {
		return err
	}
	defer f.Close()

	tmpPath := fs.logPath + ".tmp"
	tmp, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer tmp.Close()

	r := bufio.NewReader(f)
	w := bufio.NewWriter(tmp)
	for {
		e, err := readEntry(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if e.Index > index {
			break
		}
		if err := writeEntry(w, e); err != nil {
			return err
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	return os.Rename(tmpPath, fs.logPath)
}

func (fs *FileStorage) scanLastIndex() (uint64, error) {
	f, err := os.Open(fs.logPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	var last uint64
	for {
		e, err := readEntry(r)
		if err == io.EOF {
			return last, nil
		}
		if err != nil {
			return 0, err
		}
		last = e.Index
	}
}

func writeEntry(w *bufio.Writer, e LogEntry) error {
	if err := binary.Write(w, binary.LittleEndian, e.Term); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, e.Index); err != nil {
		return err
	}
	var n uint64 = uint64(len(e.Data))
	if err := binary.Write(w, binary.LittleEndian, n); err != nil {
		return err
	}
	if n > 0 {
		if _, err := w.Write(e.Data); err != nil {
			return err
		}
	}
	return nil
}

func readEntry(r *bufio.Reader) (LogEntry, error) {
	var e LogEntry
	if err := binary.Read(r, binary.LittleEndian, &e.Term); err != nil {
		return LogEntry{}, err
	}
	if err := binary.Read(r, binary.LittleEndian, &e.Index); err != nil {
		return LogEntry{}, err
	}
	var n uint64
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return LogEntry{}, err
	}
	if n > 0 {
		e.Data = make([]byte, n)
		if _, err := io.ReadFull(r, e.Data); err != nil {
			return LogEntry{}, err
		}
	} else {
		e.Data = nil
	}
	return e, nil
}
