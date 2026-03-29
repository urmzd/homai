package app

import (
	"context"
	"encoding/hex"
	"fmt"
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
			if entries := configToLoadEntries(cfg); len(entries) > 0 {
				zbController.LoadDevices(entries)
				log.Info().Int("count", len(entries)).Msg("Loaded persisted devices (NodeID assigned on rejoin)")
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

// configToLoadEntries converts persisted config devices into LoadEntry values
// for pre-populating the controller's device map on startup.
func configToLoadEntries(cfg *config.Config) []zigbee.LoadEntry {
	entries := make([]zigbee.LoadEntry, 0, len(cfg.Devices))
	for _, d := range cfg.Devices {
		addr, err := parseIEEE(d.IEEEAddress)
		if err != nil {
			log.Warn().Str("ieee", d.IEEEAddress).Err(err).Msg("Skipping device with invalid IEEE address")
			continue
		}
		entries = append(entries, zigbee.LoadEntry{
			IEEEAddress:  addr,
			FriendlyName: d.FriendlyName,
			DeviceType:   d.Type,
			Endpoint:     d.Endpoint,
			Clusters:     d.Clusters,
		})
	}
	return entries
}

// parseIEEE converts a colon-separated IEEE address string (e.g. "ff:ff:b4:0e:06:07:77:37")
// to an [8]byte in little-endian order (matching formatIEEE in the zigbee package).
func parseIEEE(s string) ([8]byte, error) {
	var addr [8]byte
	b, err := hex.DecodeString(strings.ReplaceAll(s, ":", ""))
	if err != nil || len(b) != 8 {
		return addr, fmt.Errorf("invalid IEEE address: %s", s)
	}
	// formatIEEE prints bytes 7..0, so the string is big-endian; reverse to little-endian.
	for i := range 8 {
		addr[i] = b[7-i]
	}
	return addr, nil
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

