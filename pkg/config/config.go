package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.yaml.in/yaml/v3"
)

const configFileName = "zigbee-skill.yaml"

// Config is the top-level configuration persisted to zigbee-skill.yaml.
type Config struct {
	Serial  SerialConfig  `yaml:"serial"`
	Devices []DeviceEntry `yaml:"devices"`

	mu   sync.Mutex
	path string // resolved file path for save-back
}

// SerialConfig holds the Zigbee adapter serial port settings.
type SerialConfig struct {
	Port string `yaml:"port,omitempty"`
}

// DeviceEntry is a persisted device record.
type DeviceEntry struct {
	IEEEAddress  string    `yaml:"ieee_address"`
	FriendlyName string    `yaml:"friendly_name"`
	Type         string    `yaml:"type"`
	Manufacturer string    `yaml:"manufacturer,omitempty"`
	Model        string    `yaml:"model,omitempty"`
	LastSeen     time.Time `yaml:"last_seen,omitempty"`
}

// Load reads a config file from path. If path is empty, it searches the
// default locations (./zigbee-skill.yaml then ~/.config/zigbee-skill/).
// Returns a default config if no file is found.
func Load(path string) (*Config, error) {
	if path == "" {
		path = findConfig()
	}

	cfg := &Config{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // empty default config
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes the config back to the file it was loaded from.
// Uses atomic write (temp file + rename) to avoid corruption.
func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp, c.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename config: %w", err)
	}
	return nil
}

// Path returns the resolved config file path.
func (c *Config) Path() string { return c.path }

// findConfig returns the first existing config path, or the default path.
func findConfig() string {
	// 1. Current directory
	if _, err := os.Stat(configFileName); err == nil {
		return configFileName
	}
	// 2. Global config dir
	if dir, err := globalConfigDir(); err == nil {
		p := filepath.Join(dir, configFileName)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// 3. Default: current directory
	return configFileName
}

func globalConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "zigbee-skill"), nil
}
