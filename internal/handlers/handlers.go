package handlers

import (
	"github.com/elpic/blueprint/internal/parser"
)

// Handler is the interface that all command handlers must implement
type Handler interface {
	// Up executes the action (install, clone, decrypt, etc.)
	// Returns the output message and any error
	Up() (string, error)

	// Down removes/uninstalls the resource
	// Returns the output message and any error
	Down() (string, error)
}

// BaseHandler contains common fields for all handlers
type BaseHandler struct {
	Rule     parser.Rule
	BasePath string // For resolving relative paths
}
