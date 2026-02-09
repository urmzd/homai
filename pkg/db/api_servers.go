package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrAPIServerNotFound = errors.New("api server config not found")

// APIServer represents API server configuration.
type APIServer struct {
	ID        int64
	ProfileID int64
	Host      string
	Port      int
	CreatedAt time.Time
}

// Address returns the API server listen address (host:port).
func (a *APIServer) Address() string {
	return fmt.Sprintf("%s:%d", a.Host, a.Port)
}

// APIServerStore provides API server config CRUD operations.
type APIServerStore interface {
	Get(ctx context.Context, profileID int64) (*APIServer, error)
	Create(ctx context.Context, a *APIServer) error
	Update(ctx context.Context, a *APIServer) error
	Delete(ctx context.Context, profileID int64) error
}

// APIServers returns an APIServerStore for this database.
func (db *DB) APIServers() APIServerStore {
	return &apiServerStore{db: db}
}

type apiServerStore struct {
	db *DB
}

func (s *apiServerStore) Get(ctx context.Context, profileID int64) (*APIServer, error) {
	a := &APIServer{}
	var createdAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, profile_id, host, port, created_at
		FROM api_servers WHERE profile_id = ?
	`, profileID).Scan(&a.ID, &a.ProfileID, &a.Host, &a.Port, &createdAt)
	if err == sql.ErrNoRows {
		return nil, ErrAPIServerNotFound
	}
	if err != nil {
		return nil, err
	}
	a.CreatedAt, _ = time.Parse(time.DateTime, createdAt)
	return a, nil
}

func (s *apiServerStore) Create(ctx context.Context, a *APIServer) error {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO api_servers (profile_id, host, port)
		VALUES (?, ?, ?)
	`, a.ProfileID, a.Host, a.Port)
	if err != nil {
		return fmt.Errorf("failed to create API server config: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	a.ID = id
	return nil
}

func (s *apiServerStore) Update(ctx context.Context, a *APIServer) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE api_servers SET host = ?, port = ?
		WHERE profile_id = ?
	`, a.Host, a.Port, a.ProfileID)
	return err
}

func (s *apiServerStore) Delete(ctx context.Context, profileID int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM api_servers WHERE profile_id = ?`, profileID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrAPIServerNotFound
	}
	return nil
}
