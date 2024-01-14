package main

import (
    "context"
    "golang.org/x/crypto/bcrypt"
)

type User struct {
    ID           int64  `json:"id"`
    Username     string `json:"username"`
    Email        string `json:"email"`
    PasswordHash string `json:"-"`
}

func (u *User) SaveUser(ctx context.Context, datastore db.Datastore) error {
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.PasswordHash), bcrypt.DefaultCost)
    if err != nil {
        return err
    }
    u.PasswordHash = string(hashedPassword)

    _, err = datastore.Query("INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3)", u.Username, u.Email, u.PasswordHash)
    return err
}

func AuthenticateUser(ctx context.Context, datastore db.Datastore, email, password string) (*User, error) {
    var u User
    row, err := datastore.Query("SELECT id, username, email, password_hash FROM users WHERE email = $1", email)
    if err != nil {
        return nil, err
    }
    defer row.Close()

    if row.Next() {
        err := row.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash)
        if err != nil {
            return nil, err
        }
    } else {
        return nil, sql.ErrNoRows
    }

    err = bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
    if err != nil {
        return nil, err
    }

    return &u, nil
}
