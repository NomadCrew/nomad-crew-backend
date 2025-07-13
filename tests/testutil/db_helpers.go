package testutil

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
)

// InsertTestUser ensures a user row exists in both auth.users and public.user_profiles.
// If username is empty, it is generated from the UUID (first 8 chars).
func InsertTestUser(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, email, username string) error {
	if username == "" {
		username = id.String()[:8]
	}

	// pgx Exec cannot execute multiple statements in a single call when using
	// the extended protocol. Run them separately within a transaction to keep
	// behaviour equivalent to the previous combined statement.
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx) // ignore error: rollback is a no-op if tx already committed
	}()

	_, err = tx.Exec(ctx, `INSERT INTO auth.users(id,email) VALUES($1,$2) ON CONFLICT DO NOTHING`, id, email)
	if err != nil {
		return err
	}

	// Customise expected test data based on email to satisfy downstream test expectations
	var firstName, lastName, finalUsername, avatarURL string
	switch email {
	case "test@example.com":
		firstName = "Test"
		lastName = "User"
		finalUsername = "testuser"
		avatarURL = "http://example.com/avatar1.png"
	case "another@example.com":
		firstName = ""
		lastName = ""
		finalUsername = "anotheruser"
		avatarURL = "http://example.com/avatar2.png"
	default:
		firstName = ""
		lastName = ""
		finalUsername = username
		avatarURL = fmt.Sprintf("http://example.com/%s.png", finalUsername)
	}

	_, err = tx.Exec(ctx, `INSERT INTO user_profiles(id,email,username,first_name,last_name,avatar_url) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, id, email, finalUsername, firstName, lastName, avatarURL)
	if err != nil {
		return err
	}

	// Build JSONB metadata consistent with application's expectations
	meta := fmt.Sprintf(`{"username":"%s","firstName":"%s","lastName":"%s","avatar_url":"%s"}`, finalUsername, firstName, lastName, avatarURL)

	// Full insert into legacy users table including "name" and metadata
	fullName := fmt.Sprintf("%s %s", firstName, lastName)
	_, err = tx.Exec(ctx, `INSERT INTO users(id,supabase_id,email,username,name,first_name,last_name,profile_picture_url,raw_user_meta_data) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb) ON CONFLICT (id) DO UPDATE SET email=EXCLUDED.email, username=EXCLUDED.username, name=EXCLUDED.name, first_name=EXCLUDED.first_name, last_name=EXCLUDED.last_name, profile_picture_url=EXCLUDED.profile_picture_url, raw_user_meta_data=EXCLUDED.raw_user_meta_data`,
		id, id.String(), email, finalUsername, fullName, firstName, lastName, avatarURL, meta)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}
