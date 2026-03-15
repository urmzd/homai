package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urmzd/zigbee-rest/pkg/api"
	"github.com/urmzd/zigbee-rest/pkg/app"

	_ "github.com/urmzd/zigbee-rest/docs"
)

// @title           Zigbee REST API
// @version         1.0
// @description     REST API for controlling smart home devices

// @host      localhost:8080
// @BasePath  /api/v1
// @schemes   http https

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	dbPath := flag.String("db", "", "Path to database file (default: ~/.config/zigbee-rest/zigbee-rest.db)")
	serialPort := flag.String("port", "/dev/cu.SLAB_USBtoUART", "Path to Zigbee serial port")
	flag.Parse()

	ctx := context.Background()

	a, err := app.New(ctx, *dbPath, *serialPort)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize")
	}
	defer a.Close()

	log.Info().
		Str("profile", a.Config.Profile.Name).
		Str("timezone", a.Config.Timezone()).
		Str("api_address", a.Config.APIAddress()).
		Msg("Configuration loaded")

	router := api.NewRouter(a.Controller, a.Events, a.Validator)

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Info().Msg("Shutting down...")
		a.Close()
		os.Exit(0)
	}()

	addr := a.Config.APIAddress()
	log.Info().Str("address", addr).Msg("Starting API server")

	if err := router.Run(addr); err != nil {
		log.Fatal().Err(err).Msg("Server failed")
	}
}
