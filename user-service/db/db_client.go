package db

import (
    "github.com/jackc/pgx/v4/pgxpool"
)

type DatabaseClient struct {
    pool *pgxpool.Pool
}

func NewDatabaseClient(pool *pgxpool.Pool) *DatabaseClient {
    return &DatabaseClient{pool: pool}
}

func (dc *DatabaseClient) GetPool() *pgxpool.Pool {
    return dc.pool
}
