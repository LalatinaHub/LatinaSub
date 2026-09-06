package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/LalatinaHub/LatinaSub/pkg/logger"
	"github.com/LalatinaHub/common/model"
	"github.com/LalatinaHub/common/subconverter"
)

// Exporter manages exporting verified proxy nodes into file subscriptions (sing-box, Clash, raw URLs, Base64).
type Exporter struct {
	outputDir string
}

// NewExporter creates a new Exporter instance pointing to the specified directory.
func NewExporter(outputDir string) *Exporter {
	if outputDir == "" {
		outputDir = "./result"
	}
	return &Exporter{outputDir: outputDir}
}

// OutputDir returns the configured output directory.
func (e *Exporter) OutputDir() string {
	return e.outputDir
}

// ExportAll exports the given proxy nodes into multiple subscription formats in outputDir:
// - <outputDir>/nodes: Newline-separated raw proxy URIs
// - <outputDir>/sub: Base64 encoded raw proxy URIs
// - <outputDir>/singbox.json: Complete sing-box configuration
// - <outputDir>/clash.yaml: Complete Clash Meta / Mihomo configuration
func (e *Exporter) ExportAll(nodes []*model.ProxyNode) error {
	if err := os.MkdirAll(e.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create export directory %s: %w", e.outputDir, err)
	}

	derefNodes := make([]model.ProxyNode, 0, len(nodes))
	for _, n := range nodes {
		if n != nil {
			derefNodes = append(derefNodes, *n)
		}
	}

	conv := subconverter.New(derefNodes)

	// 1. Export raw newline-delimited URLs
	rawContent := conv.ToRawString()
	rawPath := filepath.Join(e.outputDir, "nodes")
	if err := os.WriteFile(rawPath, []byte(rawContent), 0644); err != nil {
		return fmt.Errorf("failed to write raw nodes file: %w", err)
	}

	// 2. Export Base64 subscription
	b64Content := conv.ToBase64()
	subPath := filepath.Join(e.outputDir, "sub")
	if err := os.WriteFile(subPath, []byte(b64Content), 0644); err != nil {
		return fmt.Errorf("failed to write base64 sub file: %w", err)
	}

	// 3. Export sing-box JSON configuration
	singboxJSON, err := conv.ToSingbox("standard", "")
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to generate sing-box configuration")
	} else {
		singboxPath := filepath.Join(e.outputDir, "singbox.json")
		if err := os.WriteFile(singboxPath, []byte(singboxJSON), 0644); err != nil {
			return fmt.Errorf("failed to write singbox.json: %w", err)
		}
	}

	// 4. Export Clash Meta YAML configuration
	clashYAML, err := conv.ToClash("")
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to generate Clash configuration")
	} else {
		clashPath := filepath.Join(e.outputDir, "clash.yaml")
		if err := os.WriteFile(clashPath, []byte(clashYAML), 0644); err != nil {
			return fmt.Errorf("failed to write clash.yaml: %w", err)
		}
	}

	logger.Info().
		Int("node_count", len(derefNodes)).
		Str("dir", e.outputDir).
		Msg("Exported subscription files successfully")

	return nil
}
