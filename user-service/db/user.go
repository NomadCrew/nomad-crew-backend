package db

import (
    "context"
    "database/sql"
    "errors"
    "github.com/jackc/pgx/v4/pgxpool"
    "github.com/NomadCrew/nomad-crew-backend/user-service/logger"
    "github.com/NomadCrew/nomad-crew-backend/user-service/models"
)

type UserDB struct {
    pool *pgxpool.Pool
}

func NewUserDB(pool *pgxpool.Pool) *UserDB {
    return &UserDB{pool: pool}
}

func (udb *UserDB) GetPool() *pgxpool.Pool {
    return udb.pool
}

func (udb *UserDB) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
    log := logger.GetLogger()
    const query = `
        SELECT u.id, u.username, u.email, u.password_hash, u.first_name, 
               u.last_name, u.profile_picture, u.phone_number, u.address
        FROM users u
        LEFT JOIN metadata m ON m.table_name = 'users' AND m.record_id = u.id
        WHERE u.id = $1 AND m.deleted_at IS NULL`

    var user models.User
    err := udb.pool.QueryRow(ctx, query, id).Scan(
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
        log.Errorw("Failed to get user", "userId", id, "error", err)
        return nil, err
    }
    return &user, nil
}

func (udb *UserDB) SaveUser(ctx context.Context, user *models.User) error {
    log := logger.GetLogger()
    
    const query = `
        INSERT INTO users (
            username, email, password_hash, first_name, last_name,
            profile_picture, phone_number, address
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING id`

    err := udb.pool.QueryRow(
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
    
    _, err = udb.pool.Exec(ctx, metadataQuery, "users", user.ID)
    if err != nil {
        log.Errorw("Failed to save user metadata", "error", err)
        return err
    }

    return nil
}

