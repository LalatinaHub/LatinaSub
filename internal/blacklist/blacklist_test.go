package blacklist

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/LalatinaHub/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlacklistInMemoryOperations(t *testing.T) {
	bl := New()

	assert.Equal(t, 0, bl.Size())
	assert.False(t, bl.Contains("hash1"))

	// Add hash
	assert.True(t, bl.Add("hash1"))
	assert.False(t, bl.Add("hash1"), "Adding duplicate should return false")
	assert.Equal(t, 1, bl.Size())
	assert.True(t, bl.Contains("hash1"))

	// Empty hash
	assert.False(t, bl.Add(""))
	assert.False(t, bl.Contains(""))

	// Add node
	node := &model.ProxyNode{
		VPN:        "trojan",
		Server:     "1.1.1.1",
		ServerPort: 443,
		Password:   "password123",
	}

	hash, added := bl.AddNode(node)
	assert.NotEmpty(t, hash)
	assert.True(t, added)
	assert.True(t, bl.ContainsNode(node))

	// Second add should return added=false
	hash2, added2 := bl.AddNode(node)
	assert.Equal(t, hash, hash2)
	assert.False(t, added2)

	// Hashes slice
	hashes := bl.Hashes()
	assert.Len(t, hashes, 2)
	assert.Contains(t, hashes, "hash1")
	assert.Contains(t, hashes, hash)

	// Clear
	bl.Clear()
	assert.Equal(t, 0, bl.Size())
	assert.False(t, bl.Contains("hash1"))
}

func TestBlacklistConcurrency(t *testing.T) {
	bl := New(1000)
	var wg sync.WaitGroup
	workers := 20
	iterations := 100

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				h := fmt.Sprintf("worker-%d-hash-%d", workerID, j)
				bl.Add(h)
				_ = bl.Contains(h)
			}
		}(i)
	}

	wg.Wait()
	assert.Equal(t, workers*iterations, bl.Size())
}

func TestBlacklistLoadAndSave(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "blacklist.txt")

	// Pre-create file with 3 hashes
	initialContent := "hash_a\nhash_b\n# comment\n\nhash_c\n"
	require.NoError(t, os.WriteFile(path, []byte(initialContent), 0644))

	bl := New()
	loaded, err := bl.Load(path)
	require.NoError(t, err)
	assert.Equal(t, 3, loaded)
	assert.Equal(t, 3, bl.Size())
	assert.True(t, bl.Contains("hash_a"))
	assert.True(t, bl.Contains("hash_b"))
	assert.True(t, bl.Contains("hash_c"))
	assert.False(t, bl.Contains("# comment"))

	// Add new hash and save
	bl.Add("hash_d")
	require.NoError(t, bl.Save())

	// Reload in a fresh instance
	bl2 := New()
	loaded2, err := bl2.Load(path)
	require.NoError(t, err)
	assert.Equal(t, 4, loaded2)
	assert.True(t, bl2.Contains("hash_d"))
}

func TestBlacklistPrune(t *testing.T) {
	bl := New()
	for i := 0; i < 100; i++ {
		bl.Add(fmt.Sprintf("hash-%d", i))
	}
	assert.Equal(t, 100, bl.Size())

	// Prune to 70 entries
	pruned := bl.Prune(70)
	assert.Equal(t, 30, pruned)
	assert.Equal(t, 70, bl.Size())

	// Prune with larger limit does nothing
	assert.Equal(t, 0, bl.Prune(100))
	assert.Equal(t, 70, bl.Size())
}

func TestHasher(t *testing.T) {
	h1 := HashString("hello world")
	assert.Len(t, h1, 32)

	h2 := HashSHA256("hello world")
	assert.Len(t, h2, 64)

	node1 := &model.ProxyNode{
		VPN:        "vless",
		Server:     "example.com",
		ServerPort: 443,
		UUID:       "1234",
		Remark:     "Node A",
	}

	node2 := &model.ProxyNode{
		VPN:        "VLESS",
		Server:     "EXAMPLE.COM",
		ServerPort: 443,
		UUID:       "1234",
		Remark:     "Node B Different Remark",
	}

	// Remark difference should not change hash
	assert.Equal(t, HashNode(node1), HashNode(node2))
	assert.Empty(t, HashNode(nil))
}
