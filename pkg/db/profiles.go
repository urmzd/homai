package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrProfileNotFound = errors.New("profile not found")

// Profile represents a configuration profile.
type Profile struct {
	ID        int64
	Name      string
	Timezone  string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ProfileStore provides profile CRUD operations.
type ProfileStore interface {
	Get(ctx context.Context, id int64) (*Profile, error)
	GetByName(ctx context.Context, name string) (*Profile, error)
	GetActive(ctx context.Context) (*Profile, error)
	List(ctx context.Context) ([]*Profile, error)
	Create(ctx context.Context, p *Profile) error
	Update(ctx context.Context, p *Profile) error
	SetActive(ctx context.Context, id int64) error
	Delete(ctx context.Context, id int64) error
}

// Profiles returns a ProfileStore for this database.
func (db *DB) Profiles() ProfileStore {
	return &profileStore{db: db}
}

type profileStore struct {
	db *DB
}

func (s *profileStore) Get(ctx context.Context, id int64) (*Profile, error) {
	p := &Profile{}
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, timezone, is_active, created_at, updated_at
		FROM profiles WHERE id = ?
	`, id).Scan(&p.ID, &p.Name, &p.Timezone, &p.IsActive, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrProfileNotFound
	}
	if err != nil {
		return nil, err
	}
	p.CreatedAt, _ = time.Parse(time.DateTime, createdAt)
	p.UpdatedAt, _ = time.Parse(time.DateTime, updatedAt)
	return p, nil
}

func (s *profileStore) GetByName(ctx context.Context, name string) (*Profile, error) {
	p := &Profile{}
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, timezone, is_active, created_at, updated_at
		FROM profiles WHERE name = ?
	`, name).Scan(&p.ID, &p.Name, &p.Timezone, &p.IsActive, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrProfileNotFound
	}
	if err != nil {
		return nil, err
	}
	p.CreatedAt, _ = time.Parse(time.DateTime, createdAt)
	p.UpdatedAt, _ = time.Parse(time.DateTime, updatedAt)
	return p, nil
}

func (s *profileStore) GetActive(ctx context.Context) (*Profile, error) {
	p := &Profile{}
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, timezone, is_active, created_at, updated_at
		FROM profiles WHERE is_active = 1 LIMIT 1
	`).Scan(&p.ID, &p.Name, &p.Timezone, &p.IsActive, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrProfileNotFound
	}
	if err != nil {
		return nil, err
	}
	p.CreatedAt, _ = time.Parse(time.DateTime, createdAt)
	p.UpdatedAt, _ = time.Parse(time.DateTime, updatedAt)
	return p, nil
}

func (s *profileStore) List(ctx context.Context) ([]*Profile, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, timezone, is_active, created_at, updated_at
		FROM profiles ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var profiles []*Profile
	for rows.Next() {
		p := &Profile{}
		var createdAt, updatedAt string
		if err := rows.Scan(&p.ID, &p.Name, &p.Timezone, &p.IsActive, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		p.CreatedAt, _ = time.Parse(time.DateTime, createdAt)
		p.UpdatedAt, _ = time.Parse(time.DateTime, updatedAt)
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

func (s *profileStore) Create(ctx context.Context, p *Profile) error {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO profiles (name, timezone, is_active)
		VALUES (?, ?, ?)
	`, p.Name, p.Timezone, p.IsActive)
	if err != nil {
		return fmt.Errorf("failed to create profile: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	p.ID = id
	return nil
}

func (s *profileStore) Update(ctx context.Context, p *Profile) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE profiles SET name = ?, timezone = ?, is_active = ?, updated_at = datetime('now')
		WHERE id = ?
	`, p.Name, p.Timezone, p.IsActive, p.ID)
	return err
}

func (s *profileStore) SetActive(ctx context.Context, id int64) error {
	return s.db.Tx(ctx, func(tx *sql.Tx) error {
		// Deactivate all profiles
		if _, err := tx.ExecContext(ctx, `UPDATE profiles SET is_active = 0`); err != nil {
			return err
		}
		// Activate the specified profile
		result, err := tx.ExecContext(ctx, `UPDATE profiles SET is_active = 1 WHERE id = ?`, id)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return ErrProfileNotFound
		}
		return nil
	})
}

func (s *profileStore) Delete(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM profiles WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrProfileNotFound
	}
	return nil
}
