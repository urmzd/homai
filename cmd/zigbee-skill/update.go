package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

func updateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Self-update to the latest release",
		RunE:  runUpdate,
	}
}

func runUpdate(cmd *cobra.Command, args []string) error {
	const repo = "urmzd/zigbee-skill"

	// Fetch latest release metadata.
	releaseURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	resp, err := http.Get(releaseURL) //nolint:noctx
	if err != nil {
		return fmt.Errorf("fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("parse release info: %w", err)
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(version, "v")

	if current != "dev" && current == latest {
		return output(map[string]any{
			"status":  "up-to-date",
			"version": version,
		})
	}

	// Determine platform asset name.
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	// Normalize arch names to match release asset conventions.
	if goarch == "amd64" {
		goarch = "amd64"
	} else if goarch == "arm64" {
		goarch = "arm64"
	}

	asset := fmt.Sprintf("zigbee-skill-%s-%s", goos, goarch)
	downloadURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, release.TagName, asset)

	fmt.Fprintf(os.Stderr, "Downloading %s...\n", downloadURL)

	dlResp, err := http.Get(downloadURL) //nolint:noctx
	if err != nil {
		return fmt.Errorf("download binary: %w", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d (asset %q may not exist for this platform)", dlResp.StatusCode, asset)
	}

	// Write to a temp file next to the current executable.
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}

	tmp, err := os.CreateTemp("", "zigbee-skill-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) //nolint:errcheck

	if _, err := io.Copy(tmp, dlResp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("write binary: %w", err)
	}
	tmp.Close()

	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return fmt.Errorf("chmod binary: %w", err)
	}

	// Atomically replace the current executable.
	if err := os.Rename(tmpPath, exe); err != nil {
		return fmt.Errorf("replace executable: %w", err)
	}

	return output(map[string]any{
		"status":   "updated",
		"previous": version,
		"version":  release.TagName,
	})
}
