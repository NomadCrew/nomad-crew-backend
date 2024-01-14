package db

import (
	"database/sql"

	_ "github.com/lib/pq"
)

type Datastore interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

type DB struct {
	*sql.DB
}

func NewDB(driver, dataSource string) *DB {
	db, err := sql.Open(driver, dataSource)
	if err != nil {
		panic(err)
	}

	err = db.Ping()
	if err != nil {
		panic(err)
	}

	return &DB{db}
}

func (db *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return db.DB.Query(query, args...)
}
