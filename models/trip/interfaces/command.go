package interfaces

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/types"
)

// Unified CommandResult definition
type CommandResult struct {
	Success bool
	Error   error
	Data    interface{}
	Events  []types.Event
}

type Command interface {
	Execute(ctx context.Context) (*CommandResult, error)
	Validate(ctx context.Context) error
	ValidatePermissions(ctx context.Context) error
}
