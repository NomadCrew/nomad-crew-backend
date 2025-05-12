// This file is deprecated and should be removed
// Use types.DatabaseTransaction instead

package store

// DatabaseTransaction represents a database transaction
type DatabaseTransaction interface {
	Commit() error
	Rollback() error
}
