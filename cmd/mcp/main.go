package main

import (
	"context"
	"flag"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urmzd/homai/pkg/db"
	"github.com/urmzd/homai/pkg/device"
	"github.com/urmzd/homai/pkg/device/schema"
	homaimcp "github.com/urmzd/homai/pkg/mcp"
)

func main() {
	// Logging must go to stderr — stdout is the MCP transport
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Parse flags
	dbPath := flag.String("db", "", "Path to database file (default: ~/.config/homai/homai.db)")
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

	// Use NullController until a custom adapter is implemented
	controller := device.NewNullController()
	validator := schema.NewValidator()

	// Create and start MCP server
	mcpServer := homaimcp.NewServer(controller, validator)

	log.Info().Msg("Starting MCP server on stdio")

	if err := mcpServer.ServeStdio(); err != nil {
		log.Fatal().Err(err).Msg("MCP server failed")
	}
}
