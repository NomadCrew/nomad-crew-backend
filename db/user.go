package db

import (
	"context"
	"database/sql"
	"errors"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v4/pgxpool"
)

type UserDB struct {
	client *DatabaseClient
}

func NewUserDB(client *DatabaseClient) *UserDB {
	return &UserDB{client: client}
}

func (udb *UserDB) GetPool() *pgxpool.Pool {
	return udb.client.GetPool()
}

func (udb *UserDB) SaveUser(ctx context.Context, user *types.User) error {
	log := logger.GetLogger()

	const query = `
        INSERT INTO users (
            username, email, password_hash, first_name, last_name,
            profile_picture, phone_number, address
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING id`

	err := udb.client.GetPool().QueryRow(
		ctx,
		query,
		user.Username,
		user.Email,
		user.PasswordHash,
		user.FirstName,
		user.LastName,
		user.ProfilePicture,
		user.PhoneNumber,
		user.Address,
	).Scan(&user.ID)

	if err != nil {
		log.Errorw("Failed to save user", "error", err)
		return err
	}

	// Insert metadata
	const metadataQuery = `
        INSERT INTO metadata (table_name, record_id, created_at, updated_at)
        VALUES ($1, $2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`

	_, err = udb.client.GetPool().Exec(ctx, metadataQuery, "users", user.ID)
	if err != nil {
		log.Errorw("Failed to save user metadata", "error", err)
		return err
	}

	return nil
}

func (udb *UserDB) GetUserByID(ctx context.Context, id string) (*types.User, error) {
	log := logger.GetLogger()
	const query = `
        SELECT u.id, u.username, u.email, u.password_hash, u.first_name, 
               u.last_name, u.profile_picture, u.phone_number, u.address
        FROM users u
        LEFT JOIN metadata m ON m.table_name = 'users' AND m.record_id = u.id
        WHERE u.id = $1 AND m.deleted_at IS NULL`

	var user types.User
	err := udb.client.GetPool().QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.FirstName,
		&user.LastName,
		&user.ProfilePicture,
		&user.PhoneNumber,
		&user.Address,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found")
		}
		log.Errorw("Failed to get user", "error", err)
		return nil, err
	}

	return &user, nil
}

// UpdateUser updates an existing user
func (udb *UserDB) UpdateUser(ctx context.Context, user *types.User) error {
	log := logger.GetLogger()

	const query = `
        UPDATE users
        SET username = $1, email = $2, first_name = $3, last_name = $4,
            profile_picture = $5, phone_number = $6, address = $7
        WHERE id = $8`

	result, err := udb.client.GetPool().Exec(ctx, query,
		user.Username,
		user.Email,
		user.FirstName,
		user.LastName,
		user.ProfilePicture,
		user.PhoneNumber,
		user.Address,
		user.ID,
	)

	if err != nil {
		log.Errorw("Failed to update user", "error", err)
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("user not found")
	}

	// Update metadata
	const metadataQuery = `
        UPDATE metadata
        SET updated_at = CURRENT_TIMESTAMP
        WHERE table_name = 'users' AND record_id = $1`

	_, err = udb.client.GetPool().Exec(ctx, metadataQuery, user.ID)
	if err != nil {
		log.Errorw("Failed to update user metadata", "error", err)
		return err
	}

	return nil
}

// DeleteUser performs a soft delete of a user
func (udb *UserDB) DeleteUser(ctx context.Context, id string) error {
	log := logger.GetLogger()

	const query = `
        UPDATE metadata
        SET deleted_at = CURRENT_TIMESTAMP
        WHERE table_name = 'users' AND record_id = $1`

	result, err := udb.client.GetPool().Exec(ctx, query, id)
	if err != nil {
		log.Errorw("Failed to delete user", "error", err)
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("user not found")
	}

	return nil
}

// AuthenticateUser authenticates a user by email and password
func (udb *UserDB) AuthenticateUser(ctx context.Context, email string) (*types.User, error) {
	log := logger.GetLogger()

	const query = `
        SELECT u.id, u.username, u.email, u.password_hash, u.first_name, 
               u.last_name, u.profile_picture, u.phone_number, u.address
        FROM users u
        LEFT JOIN metadata m ON m.table_name = 'users' AND m.record_id = u.id
        WHERE u.email = $1 AND m.deleted_at IS NULL`

	var user types.User
	err := udb.client.GetPool().QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.FirstName,
		&user.LastName,
		&user.ProfilePicture,
		&user.PhoneNumber,
		&user.Address,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found")
		}
		log.Errorw("Failed to authenticate user", "error", err)
		return nil, err
	}

	return &user, nil
}
