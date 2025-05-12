package types

// DatabaseTransaction represents a database transaction
type DatabaseTransaction interface {
	Commit() error
	Rollback() error
}
