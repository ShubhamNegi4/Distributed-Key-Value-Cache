package aof

import (
	resp "Distributed_Cache/Resp"
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"hash/crc64"
	"io"
	"os"
	"sync"
	"time"
)

// FsyncPolicy defines when to sync data to disk
type FsyncPolicy int

const (
	FsyncAlways   FsyncPolicy = iota // Sync after every write
	FsyncEverySec                    // Sync every second
	FsyncNo                          // Let OS handle syncing
)

// AofConfig holds configuration for AOF
type AofConfig struct {
	FsyncPolicy    FsyncPolicy
	RewriteEnabled bool
	RewriteSize    int64         // Trigger rewrite when AOF size exceeds this
	RewriteMinAge  time.Duration // Minimum age before rewrite
}

// DefaultAofConfig returns default configuration
func DefaultAofConfig() *AofConfig {
	return &AofConfig{
		FsyncPolicy:    FsyncEverySec,
		RewriteEnabled: true,
		RewriteSize:    64 * 1024 * 1024, // 64MB
		RewriteMinAge:  time.Hour,
	}
}

type Aof struct {
	file     *os.File
	rd       *bufio.Reader
	mu       sync.RWMutex
	config   *AofConfig
	lastSync time.Time
	fileInfo os.FileInfo
	checksum uint64
}

// NewAof creates a new AOF instance with default configuration
func NewAof(path string) (*Aof, error) {
	return NewAofWithConfig(path, DefaultAofConfig())
}

// NewAofWithConfig creates a new AOF instance with custom configuration
func NewAofWithConfig(path string, config *AofConfig) (*Aof, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return nil, err
	}

	fileInfo, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	aof := &Aof{
		file:     f,
		rd:       bufio.NewReader(f),
		config:   config,
		lastSync: time.Now(),
		fileInfo: fileInfo,
		checksum: crc64.Checksum([]byte{}, crc64.MakeTable(crc64.ISO)),
	}

	// Start background sync goroutine if needed
	if config.FsyncPolicy == FsyncEverySec {
		go aof.backgroundSync()
	}

	// Start AOF rewrite monitoring if enabled
	if config.RewriteEnabled {
		go aof.monitorRewrite()
	}

	return aof, nil
}

// backgroundSync handles periodic syncing for FsyncEverySec policy
func (aof *Aof) backgroundSync() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		aof.mu.Lock()
		if time.Since(aof.lastSync) >= time.Second {
			aof.file.Sync()
			aof.lastSync = time.Now()
		}
		aof.mu.Unlock()
	}
}

// monitorRewrite monitors AOF size and triggers rewrite when needed
func (aof *Aof) monitorRewrite() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		aof.mu.RLock()
		fileInfo, err := aof.file.Stat()
		aof.mu.RUnlock()

		if err != nil {
			continue
		}

		// Check if rewrite is needed
		if fileInfo.Size() > aof.config.RewriteSize &&
			time.Since(fileInfo.ModTime()) > aof.config.RewriteMinAge {
			go aof.rewrite()
		}
	}
}

// Write into a file with checksum and proper fsync handling
func (aof *Aof) Write(value resp.Value) error {
	aof.mu.Lock()
	defer aof.mu.Unlock()

	data := value.Marshal()

	// Add partial write marker
	marker := make([]byte, 8)
	rand.Read(marker)

	// Write marker
	if _, err := aof.file.Write(marker); err != nil {
		return err
	}

	// Write data length
	length := make([]byte, 8)
	binary.LittleEndian.PutUint64(length, uint64(len(data)))
	if _, err := aof.file.Write(length); err != nil {
		return err
	}

	// Write actual data
	if _, err := aof.file.Write(data); err != nil {
		return err
	}

	// Calculate and write checksum
	checksum := crc64.Checksum(data, crc64.MakeTable(crc64.ISO))
	checksumBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(checksumBytes, checksum)
	if _, err := aof.file.Write(checksumBytes); err != nil {
		return err
	}

	// Write completion marker (same as start marker)
	if _, err := aof.file.Write(marker); err != nil {
		return err
	}

	// Update internal checksum
	aof.checksum = crc64.Update(aof.checksum, crc64.MakeTable(crc64.ISO), data)

	// Handle fsync based on policy
	switch aof.config.FsyncPolicy {
	case FsyncAlways:
		if err := aof.file.Sync(); err != nil {
			return err
		}
		aof.lastSync = time.Now()
	case FsyncEverySec:
		// Background goroutine handles this
	case FsyncNo:
		// Let OS handle it
	}

	return nil
}

// Read from the file and replay commands with corruption handling
func (aof *Aof) Read(fn func(resp.Value)) error {
	aof.mu.Lock()
	defer aof.mu.Unlock()

	aof.file.Seek(0, 0)

	for {
		// Read start marker
		startMarker := make([]byte, 8)
		if _, err := io.ReadFull(aof.file, startMarker); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		// Read data length
		lengthBytes := make([]byte, 8)
		if _, err := io.ReadFull(aof.file, lengthBytes); err != nil {
			if err == io.EOF {
				// Truncate at corruption point
				aof.truncateAtCorruption()
				break
			}
			return err
		}

		dataLength := binary.LittleEndian.Uint64(lengthBytes)

		// Read actual data
		data := make([]byte, dataLength)
		if _, err := io.ReadFull(aof.file, data); err != nil {
			if err == io.EOF {
				// Truncate at corruption point
				aof.truncateAtCorruption()
				break
			}
			return err
		}

		// Read checksum
		checksumBytes := make([]byte, 8)
		if _, err := io.ReadFull(aof.file, checksumBytes); err != nil {
			if err == io.EOF {
				// Truncate at corruption point
				aof.truncateAtCorruption()
				break
			}
			return err
		}

		expectedChecksum := binary.LittleEndian.Uint64(checksumBytes)
		actualChecksum := crc64.Checksum(data, crc64.MakeTable(crc64.ISO))

		// Verify checksum
		if expectedChecksum != actualChecksum {
			// Truncate at corruption point
			aof.truncateAtCorruption()
			break
		}

		// Read end marker
		endMarker := make([]byte, 8)
		if _, err := io.ReadFull(aof.file, endMarker); err != nil {
			if err == io.EOF {
				// Truncate at corruption point
				aof.truncateAtCorruption()
				break
			}
			return err
		}

		// Verify markers match
		if !bytesEqual(startMarker, endMarker) {
			// Truncate at corruption point
			aof.truncateAtCorruption()
			break
		}

		// Parse and execute the command
		reader := resp.NewResp(bytes.NewReader(data))
		value, err := reader.Read()
		if err != nil {
			// Truncate at corruption point
			aof.truncateAtCorruption()
			break
		}

		fn(value)
	}

	return nil
}

// truncateAtCorruption truncates the AOF file at the current position
func (aof *Aof) truncateAtCorruption() {
	currentPos, _ := aof.file.Seek(0, io.SeekCurrent)
	aof.file.Truncate(currentPos)
	aof.file.Sync()
}

// bytesEqual compares two byte slices
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// rewrite performs AOF rewrite to compact the file
func (aof *Aof) rewrite() error {
	aof.mu.Lock()
	defer aof.mu.Unlock()

	// Create temporary file for rewrite
	tempFile, err := os.CreateTemp("", "aof_rewrite_*")
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Reset to beginning
	aof.file.Seek(0, 0)

	// Read all valid commands and write to temp file
	tempWriter := bufio.NewWriter(tempFile)

	for {
		// Read start marker
		startMarker := make([]byte, 8)
		if _, err := io.ReadFull(aof.file, startMarker); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		// Read data length
		lengthBytes := make([]byte, 8)
		if _, err := io.ReadFull(aof.file, lengthBytes); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		dataLength := binary.LittleEndian.Uint64(lengthBytes)

		// Read actual data
		data := make([]byte, dataLength)
		if _, err := io.ReadFull(aof.file, data); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		// Read checksum
		checksumBytes := make([]byte, 8)
		if _, err := io.ReadFull(aof.file, checksumBytes); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		expectedChecksum := binary.LittleEndian.Uint64(checksumBytes)
		actualChecksum := crc64.Checksum(data, crc64.MakeTable(crc64.ISO))

		// Verify checksum
		if expectedChecksum != actualChecksum {
			break // Stop at corruption
		}

		// Read end marker
		endMarker := make([]byte, 8)
		if _, err := io.ReadFull(aof.file, endMarker); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		// Verify markers match
		if !bytesEqual(startMarker, endMarker) {
			break // Stop at corruption
		}

		// Write to temp file (simplified format for rewrite)
		if _, err := tempWriter.Write(data); err != nil {
			return err
		}
	}

	// Flush temp file
	if err := tempWriter.Flush(); err != nil {
		return err
	}

	// Close temp file
	tempFile.Close()

	// Replace original file with rewritten version
	if err := os.Rename(tempFile.Name(), aof.file.Name()); err != nil {
		return err
	}

	// Reopen the file
	aof.file.Close()
	newFile, err := os.OpenFile(aof.file.Name(), os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return err
	}

	aof.file = newFile
	aof.rd = bufio.NewReader(aof.file)

	return nil
}

// GetConfig returns the current AOF configuration
func (aof *Aof) GetConfig() *AofConfig {
	return aof.config
}

// SetConfig updates the AOF configuration
func (aof *Aof) SetConfig(config *AofConfig) {
	aof.mu.Lock()
	defer aof.mu.Unlock()
	aof.config = config
}

// GetChecksum returns the current file checksum
func (aof *Aof) GetChecksum() uint64 {
	aof.mu.RLock()
	defer aof.mu.RUnlock()
	return aof.checksum
}

// close the file
func (aof *Aof) Close() error {
	aof.mu.Lock()
	defer aof.mu.Unlock()
	return aof.file.Close()
}
