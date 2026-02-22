package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urmzd/homai/pkg/api"
	"github.com/urmzd/homai/pkg/db"
	"github.com/urmzd/homai/pkg/device"
	"github.com/urmzd/homai/pkg/device/schema"
	"github.com/urmzd/homai/pkg/zigbee"

	_ "github.com/urmzd/homai/docs"
)

// @title           Homai API
// @version         1.0
// @description     REST API for controlling smart home devices

// @host      localhost:8080
// @BasePath  /api/v1
// @schemes   http https

func main() {
	// Configure logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Parse flags
	dbPath := flag.String("db", "", "Path to database file (default: ~/.config/homai/homai.db)")
	serialPort := flag.String("port", "/dev/cu.SLAB_USBtoUART", "Path to Zigbee serial port")
	flag.Parse()

	ctx := context.Background()

	// Open database
	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open database")
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close database")
		}
	}()

	log.Info().Str("path", database.Path()).Msg("Database opened")

	// Run migrations
	if err := database.Migrate(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to run database migrations")
	}

	// Bootstrap if needed (first run)
	needsBootstrap, err := database.NeedsBootstrap(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to check bootstrap status")
	}
	if needsBootstrap {
		log.Info().Msg("First run detected, bootstrapping database...")
		if err := database.Bootstrap(ctx); err != nil {
			log.Fatal().Err(err).Msg("Failed to bootstrap database")
		}
		log.Info().Msg("Database bootstrapped successfully")
	}

	// Load configuration
	cfg, err := database.ActiveConfig(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	log.Info().
		Str("profile", cfg.Profile.Name).
		Str("timezone", cfg.Timezone()).
		Str("api_address", cfg.APIAddress()).
		Msg("Configuration loaded")

	// Try to connect to the Zigbee dongle; fall back to NullController
	var controller device.Controller
	var eventSubscriber device.EventSubscriber

	zbController, err := zigbee.NewController(*serialPort)
	if err != nil {
		log.Warn().Err(err).Str("port", *serialPort).Msg("Zigbee controller unavailable, using null controller")
		controller = device.NewNullController()
		eventSubscriber = device.NewNullEventSubscriber()
	} else {
		controller = zbController
		eventSubscriber = zbController
	}

	validator := schema.NewValidator()

	// Create and start API router
	router := api.NewRouter(controller, eventSubscriber, validator)

	// Handle shutdown gracefully
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Info().Msg("Shutting down...")
		if err := database.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close database")
		}
		os.Exit(0)
	}()

	// Start server
	addr := cfg.APIAddress()
	log.Info().Str("address", addr).Msg("Starting API server")

	if err := router.Run(addr); err != nil {
		log.Fatal().Err(err).Msg("Server failed")
	}
}
