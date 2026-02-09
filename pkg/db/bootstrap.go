package db

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"os"
)

// Bootstrap initializes the database with default data if it's empty.
// This is called after migrations and handles first-run setup.
func (db *DB) Bootstrap(ctx context.Context) error {
	// Check if any profiles exist
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM profiles`).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check profiles: %w", err)
	}

	if count > 0 {
		return nil // Already bootstrapped
	}

	// First run - create defaults
	timezone := detectTimezone()

	// Create default profile
	result, err := db.ExecContext(ctx, `
		INSERT INTO profiles (name, timezone, is_active)
		VALUES (?, ?, 1)
	`, "default", timezone)
	if err != nil {
		return fmt.Errorf("failed to create default profile: %w", err)
	}

	profileID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get profile ID: %w", err)
	}

	// Create default API server config
	_, err = db.ExecContext(ctx, `
		INSERT INTO api_servers (profile_id, host, port)
		VALUES (?, '0.0.0.0', 8080)
	`, profileID)
	if err != nil {
		return fmt.Errorf("failed to create default API server: %w", err)
	}

	return nil
}

// detectTimezone attempts to detect the system timezone.
func detectTimezone() string {
	switch runtime.GOOS {
	case "darwin":
		// Try systemsetup first
		out, err := exec.Command("systemsetup", "-gettimezone").Output()
		if err == nil {
			parts := strings.SplitN(string(out), ": ", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}

		// Fallback: read /etc/localtime symlink
		if link, err := os.Readlink("/etc/localtime"); err == nil {
			if idx := strings.Index(link, "zoneinfo/"); idx != -1 {
				return link[idx+9:]
			}
		}

	case "linux":
		// Try timedatectl first (systemd)
		out, err := exec.Command("timedatectl", "show", "--property=Timezone", "--value").Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}

		// Fallback: /etc/timezone file
		if data, err := os.ReadFile("/etc/timezone"); err == nil {
			return strings.TrimSpace(string(data))
		}

		// Fallback: /etc/localtime symlink
		if link, err := os.Readlink("/etc/localtime"); err == nil {
			if idx := strings.Index(link, "zoneinfo/"); idx != -1 {
				return link[idx+9:]
			}
		}
	}

	return "UTC"
}

// NeedsBootstrap returns true if the database needs initial setup.
func (db *DB) NeedsBootstrap(ctx context.Context) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM profiles`).Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}
