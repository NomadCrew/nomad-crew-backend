package db

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v4/pgxpool"
)

type DatabaseClient struct {
	pool   *pgxpool.Pool
	config *pgxpool.Config
	mu     sync.RWMutex
}

func NewDatabaseClient(pool *pgxpool.Pool) *DatabaseClient {
	return &DatabaseClient{pool: pool}
}

func (dc *DatabaseClient) GetPool() *pgxpool.Pool {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	return dc.pool
}

func (dc *DatabaseClient) RefreshPool(ctx context.Context) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if dc.pool != nil {
		dc.pool.Close()
	}

	newPool, err := pgxpool.ConnectConfig(ctx, dc.config)
	if err != nil {
		return err
	}

	dc.pool = newPool
	return nil
}
