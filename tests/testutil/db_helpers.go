package testutil

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
)

// InsertTestUser ensures a user row exists in both auth.users and public.user_profiles.
// If username is empty, it is generated from the UUID (first 8 chars).
func InsertTestUser(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, email, username string) error {
	if username == "" {
		username = id.String()[:8]
	}

	sql := `
        INSERT INTO auth.users(id,email) VALUES($1,$2)
        ON CONFLICT DO NOTHING;
        INSERT INTO user_profiles(id,email,username) VALUES($1,$2,$3)
        ON CONFLICT DO NOTHING;`

	_, err := pool.Exec(ctx, sql, id, email, username)
	return err
}
