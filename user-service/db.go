package main

import (
	"context"
	"log"
	"github.com/jackc/pgx/v4/pgxpool"
)

var DbPool *pgxpool.Pool

func ConnectToDB(connectionString string) {
	var err error
	DbPool, err = pgxpool.Connect(context.Background(), connectionString)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
}
