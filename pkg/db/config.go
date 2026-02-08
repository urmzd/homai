package db

import (
	"context"
	"errors"
	"fmt"
)

var ErrNoActiveProfile = errors.New("no active profile found")

// Config represents the complete runtime configuration loaded from the database.
type Config struct {
	Profile   *Profile
	APIServer *APIServer
}

// APIAddress returns the API server listen address.
func (c *Config) APIAddress() string {
	if c.APIServer == nil {
		return "0.0.0.0:8080"
	}
	return c.APIServer.Address()
}

// Timezone returns the profile timezone.
func (c *Config) Timezone() string {
	if c.Profile == nil {
		return "UTC"
	}
	return c.Profile.Timezone
}

// ActiveConfig loads the complete configuration for the active profile.
func (db *DB) ActiveConfig(ctx context.Context) (*Config, error) {
	// Get active profile
	profile, err := db.Profiles().GetActive(ctx)
	if err != nil {
		if errors.Is(err, ErrProfileNotFound) {
			return nil, ErrNoActiveProfile
		}
		return nil, fmt.Errorf("failed to get active profile: %w", err)
	}

	config := &Config{
		Profile: profile,
	}

	// Get API server config
	apiServer, err := db.APIServers().Get(ctx, profile.ID)
	if err != nil && !errors.Is(err, ErrAPIServerNotFound) {
		return nil, fmt.Errorf("failed to get API server config: %w", err)
	}
	config.APIServer = apiServer

	return config, nil
}
