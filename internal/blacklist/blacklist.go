package blacklist

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/LalatinaHub/LatinaSub/pkg/logger"
	"github.com/LalatinaHub/common/model"
)

// Blacklist provides thread-safe, in-memory O(1) dead account checking
// with streaming buffered disk persistence and atomic file updates.
type Blacklist struct {
	mu   sync.RWMutex
	set  map[string]struct{}
	path string
}

// New creates a new Blacklist instance.
func New(initialCapacity ...int) *Blacklist {
	cap := 10000
	if len(initialCapacity) > 0 && initialCapacity[0] > 0 {
		cap = initialCapacity[0]
	}

	return &Blacklist{
		set: make(map[string]struct{}, cap),
	}
}

// Add adds a hash to the blacklist. Returns true if it was newly inserted.
func (b *Blacklist) Add(hash string) bool {
	h := strings.TrimSpace(hash)
	if h == "" {
		return false
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.set[h]; exists {
		return false
	}

	b.set[h] = struct{}{}
	return true
}

// AddNode hashes the proxy node and adds its fingerprint to the blacklist.
func (b *Blacklist) AddNode(node *model.ProxyNode) (string, bool) {
	hash := HashNode(node)
	if hash == "" {
		return "", false
	}

	added := b.Add(hash)
	return hash, added
}

// Contains checks if a hash is present in the blacklist in O(1) time.
func (b *Blacklist) Contains(hash string) bool {
	h := strings.TrimSpace(hash)
	if h == "" {
		return false
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	_, exists := b.set[h]
	return exists
}

// ContainsNode checks if a proxy node's deterministic hash is present in the blacklist.
func (b *Blacklist) ContainsNode(node *model.ProxyNode) bool {
	hash := HashNode(node)
	if hash == "" {
		return false
	}

	return b.Contains(hash)
}

// Size returns the total count of dead account hashes stored in memory.
func (b *Blacklist) Size() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.set)
}

// Count returns the total count of dead account hashes (alias to Size).
func (b *Blacklist) Count() int {
	return b.Size()
}


// Clear removes all entries from the blacklist.
func (b *Blacklist) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.set = make(map[string]struct{}, 1000)
}

// Hashes returns a slice of all stored hash strings.
func (b *Blacklist) Hashes() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	hashes := make([]string, 0, len(b.set))
	for h := range b.set {
		hashes = append(hashes, h)
	}
	return hashes
}

// Load reads blacklist entries from a file using a streaming scanner.
// If the file does not exist, it is created and 0 is returned.
func (b *Blacklist) Load(path string) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.path = path

	file, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return 0, fmt.Errorf("failed to open blacklist file %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Allocate generous buffer (up to 10MB) to prevent token too long errors
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		b.set[line] = struct{}{}
		count++
	}

	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("error reading blacklist file %s: %w", path, err)
	}

	logger.Info().
		Int("loaded_hashes", count).
		Int("total_hashes", len(b.set)).
		Str("path", path).
		Msg("Blacklist loaded successfully")

	return count, nil
}

// Save writes all blacklisted hashes to file atomically using a temporary file.
// This prevents file corruption in case of unexpected process termination.
func (b *Blacklist) Save(path ...string) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	targetPath := b.path
	if len(path) > 0 && path[0] != "" {
		targetPath = path[0]
	}
	if targetPath == "" {
		targetPath = "blacklist.txt"
	}

	// Ensure parent directory exists
	dir := filepath.Dir(targetPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory for blacklist: %w", err)
		}
	}

	// Write to temporary file first for atomic durability
	tmpPath := targetPath + ".tmp"
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open temporary blacklist file %s: %w", tmpPath, err)
	}

	writer := bufio.NewWriterSize(file, 256*1024) // 256KB buffer for efficient disk I/O
	for hash := range b.set {
		if _, err := writer.WriteString(hash + "\n"); err != nil {
			file.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("failed to write hash to temporary file: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		file.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to flush temporary blacklist buffer: %w", err)
	}

	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to close temporary blacklist file: %w", err)
	}

	// Atomic replace
	if err := os.Rename(tmpPath, targetPath); err != nil {
		// Fallback for Windows if target file already exists
		_ = os.Remove(targetPath)
		if err := os.Rename(tmpPath, targetPath); err != nil {
			return fmt.Errorf("failed to rename temporary file to %s: %w", targetPath, err)
		}
	}

	logger.Info().
		Int("saved_hashes", len(b.set)).
		Str("path", targetPath).
		Msg("Blacklist saved successfully")

	return nil
}

// Prune restricts the blacklist size to maxEntries to prevent unbounded file growth in git repositories.
// Returns the number of entries pruned.
func (b *Blacklist) Prune(maxEntries int) int {
	if maxEntries <= 0 {
		return 0
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	excess := len(b.set) - maxEntries
	if excess <= 0 {
		return 0
	}

	pruned := 0
	for k := range b.set {
		if pruned >= excess {
			break
		}
		delete(b.set, k)
		pruned++
	}

	logger.Warn().
		Int("pruned_count", pruned).
		Int("remaining_count", len(b.set)).
		Msg("Blacklist pruned to maintain storage budget")

	return pruned
}
