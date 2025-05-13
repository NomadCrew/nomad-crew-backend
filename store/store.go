package store

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/types"
)

// This file defines common store interfaces and types
// The error constants are defined in errors.go

// BeginTx starts a new transaction
func BeginTx(ctx context.Context) (types.DatabaseTransaction, error) {
	// Implementation of BeginTx
	return nil, nil // Placeholder return, actual implementation needed
}
