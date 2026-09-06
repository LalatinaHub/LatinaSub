package blacklist

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/LalatinaHub/common/model"
)

// HashNode computes a deterministic, canonical MD5 hash for a proxy node.
// It normalizes server names, protocols, and transports to lowercase, and ignores volatile fields
// such as remark/display names, latencies, and region codes.
func HashNode(node *model.ProxyNode) string {
	if node == nil {
		return ""
	}

	canonical := fmt.Sprintf("%s|%s|%d|%s|%s|%s|%s|%s|%s",
		strings.ToLower(strings.TrimSpace(node.VPN)),
		strings.ToLower(strings.TrimSpace(node.Server)),
		node.ServerPort,
		strings.TrimSpace(node.UUID),
		strings.TrimSpace(node.Password),
		strings.ToLower(strings.TrimSpace(node.Transport)),
		strings.TrimSpace(node.Path),
		strings.ToLower(strings.TrimSpace(node.SNI)),
		strings.ToLower(strings.TrimSpace(node.Method)),
	)

	return HashString(canonical)
}

// HashString returns the MD5 hex string of an input text.
func HashString(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// HashSHA256 returns the SHA256 hex string of an input text.
func HashSHA256(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
