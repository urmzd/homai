package app

import (
	"context"
	"encoding/hex"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/urmzd/zigbee-skill/pkg/config"
	"github.com/urmzd/zigbee-skill/pkg/device"
	"github.com/urmzd/zigbee-skill/pkg/device/schema"
	zigbee "github.com/urmzd/zigbee-skill/pkg/zigbee"
)

// App holds the shared core services used by the CLI.
type App struct {
	Config     *config.Config
	Controller device.Controller
	Events     device.EventSubscriber
	Validator  *schema.Validator
}

// New initializes the config, controller, and validator.
// If serialPort is empty, the config's serial.port is used.
// If neither is set, a null controller is used.
func New(_ context.Context, configPath, serialPort string) (*App, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	log.Info().Str("path", cfg.Path()).Msg("Config loaded")

	if serialPort == "" {
		serialPort = cfg.Serial.Port
	}

	var controller device.Controller
	var events device.EventSubscriber

	if serialPort != "" {
		zbController, err := zigbee.NewController(serialPort)
		if err != nil {
			log.Warn().Err(err).Str("port", serialPort).Msg("Zigbee controller unavailable, using null controller")
			controller = device.NewNullController()
			events = device.NewNullEventSubscriber()
		} else {
			// Load persisted devices into the controller
			entries := configToLoadEntries(cfg)
			zbController.LoadDevices(entries)
			if len(entries) > 0 {
				log.Info().Int("count", len(entries)).Msg("Loaded persisted devices")
			}

			// Wire persistence: save config when devices change
			zbController.SetOnDeviceChange(func() {
				syncDevicesToConfig(zbController, cfg)
				if err := cfg.Save(); err != nil {
					log.Error().Err(err).Msg("Failed to save config after device change")
				}
			})

			// Persist serial port to config if not already set
			if cfg.Serial.Port == "" {
				cfg.Serial.Port = serialPort
				_ = cfg.Save()
			}

			controller = zbController
			events = zbController
		}
	} else {
		controller = device.NewNullController()
		events = device.NewNullEventSubscriber()
	}

	return &App{
		Config:     cfg,
		Controller: controller,
		Events:     events,
		Validator:  schema.NewValidator(),
	}, nil
}

// Close releases all resources.
func (a *App) Close() {
	if a.Controller != nil {
		a.Controller.Close()
	}
}

// configToLoadEntries converts config device entries to controller load entries.
func configToLoadEntries(cfg *config.Config) []zigbee.LoadEntry {
	entries := make([]zigbee.LoadEntry, 0, len(cfg.Devices))
	for _, d := range cfg.Devices {
		ieee, err := parseIEEE(d.IEEEAddress)
		if err != nil {
			log.Warn().Str("ieee", d.IEEEAddress).Err(err).Msg("Skipping device with invalid IEEE address")
			continue
		}
		entries = append(entries, zigbee.LoadEntry{
			IEEEAddress:  ieee,
			FriendlyName: d.FriendlyName,
			DeviceType:   d.Type,
			Endpoint:     d.Endpoint,
			Clusters:     d.Clusters,
		})
	}
	return entries
}

// syncDevicesToConfig exports the controller's in-memory devices to config.
func syncDevicesToConfig(zb *zigbee.Controller, cfg *config.Config) {
	exported := zb.ExportDevices()
	cfg.Devices = make([]config.DeviceEntry, 0, len(exported))
	for _, d := range exported {
		cfg.Devices = append(cfg.Devices, config.DeviceEntry{
			IEEEAddress:  d.IEEEAddress,
			FriendlyName: d.FriendlyName,
			Type:         d.DeviceType,
			Endpoint:     d.Endpoint,
			Clusters:     d.Clusters,
			LastSeen:     time.Now(),
		})
	}
}

// parseIEEE converts a colon-separated IEEE address string to [8]byte.
// The string is big-endian (MSB first, as formatted by formatIEEE),
// but the [8]byte is little-endian (LSB at index 0).
func parseIEEE(s string) ([8]byte, error) {
	var addr [8]byte
	clean := strings.ReplaceAll(s, ":", "")
	b, err := hex.DecodeString(clean)
	if err != nil || len(b) != 8 {
		return addr, err
	}
	// Reverse: big-endian string → little-endian byte array
	for i := range 8 {
		addr[i] = b[7-i]
	}
	return addr, nil
}
