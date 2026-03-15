package app

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/urmzd/zigbee-rest/pkg/db"
	"github.com/urmzd/zigbee-rest/pkg/device"
	"github.com/urmzd/zigbee-rest/pkg/device/schema"
	"github.com/urmzd/zigbee-rest/pkg/zigbee"
)

// App holds the shared core services used by both the API server and CLI.
type App struct {
	DB         *db.DB
	Config     *db.Config
	Controller device.Controller
	Events     device.EventSubscriber
	Validator  *schema.Validator
}

// New initializes the database, controller, and validator.
// If serialPort is empty, the Zigbee controller is skipped and a null controller is used.
func New(ctx context.Context, dbPath, serialPort string) (*App, error) {
	database, err := db.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	log.Info().Str("path", database.Path()).Msg("Database opened")

	if err := database.Migrate(ctx); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	needsBootstrap, err := database.NeedsBootstrap(ctx)
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("check bootstrap: %w", err)
	}
	if needsBootstrap {
		log.Info().Msg("First run detected, bootstrapping database...")
		if err := database.Bootstrap(ctx); err != nil {
			_ = database.Close()
			return nil, fmt.Errorf("bootstrap database: %w", err)
		}
	}

	cfg, err := database.ActiveConfig(ctx)
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("load config: %w", err)
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
			controller = zbController
			events = zbController
		}
	} else {
		controller = device.NewNullController()
		events = device.NewNullEventSubscriber()
	}

	return &App{
		DB:         database,
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
	if a.DB != nil {
		if err := a.DB.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close database")
		}
	}
}
