package model

import (
	"github.com/LalatinaHub/common/model"
)

// Re-export common domain models for internal usage and consistency across LalatinaHub repositories.
type (
	// ProxyNode represents a universal parsed proxy node configuration.
	ProxyNode = model.ProxyNode

	// Server represents an edge server in the cluster.
	Server = model.Server

	// User represents a subscriber / user.
	User = model.User

	// KeyValue represents general configuration key-value pair.
	KeyValue = model.KeyValue
)
