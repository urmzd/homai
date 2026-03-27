package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urmzd/zigbee-skill/pkg/app"
	"github.com/urmzd/zigbee-skill/pkg/daemon"
	"github.com/urmzd/zigbee-skill/pkg/device"
	"github.com/urmzd/zigbee-skill/pkg/device/schema"
)

// Set via -ldflags at build time.
var version = "dev"

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.SetGlobalLevel(zerolog.WarnLevel)

	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(1)
	}

	// Extract global flags
	var configPath, serialPort string
	var socketPath, pidPath, logPath string
	var daemonForeground bool
	var filtered []string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--config" && i+1 < len(args):
			configPath = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--config="):
			configPath = args[i][len("--config="):]
		case args[i] == "--port" && i+1 < len(args):
			serialPort = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--port="):
			serialPort = args[i][len("--port="):]
		case args[i] == "--socket" && i+1 < len(args):
			socketPath = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--socket="):
			socketPath = args[i][len("--socket="):]
		case args[i] == "--pid" && i+1 < len(args):
			pidPath = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--pid="):
			pidPath = args[i][len("--pid="):]
		case args[i] == "--log" && i+1 < len(args):
			logPath = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--log="):
			logPath = args[i][len("--log="):]
		case args[i] == "--daemon-foreground":
			daemonForeground = true
		default:
			filtered = append(filtered, args[i])
		}
	}
	args = filtered

	if socketPath == "" {
		socketPath = daemon.DefaultSocketPath
	}
	if pidPath == "" {
		pidPath = daemon.DefaultPIDPath
	}
	if logPath == "" {
		logPath = daemon.DefaultLogPath
	}

	// Internal: run as foreground daemon (called by Fork).
	if daemonForeground {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		log.Info().Str("version", version).Msg("Starting zigbee-skill daemon")
		srv := daemon.NewServer(socketPath, pidPath)
		if err := srv.Start(configPath, serialPort); err != nil {
			fatal("daemon: %s", err)
		}
		return
	}

	if len(args) == 0 {
		usage()
		os.Exit(1)
	}

	// Handle daemon subcommand before initializing the app.
	if args[0] == "daemon" {
		if err := cmdDaemon(args[1:], socketPath, pidPath, logPath, configPath, serialPort); err != nil {
			fatal("%s", err)
		}
		return
	}

	ctx := context.Background()

	// Auto-detect running daemon and route through it.
	var a *app.App
	if running, _, _ := daemon.IsRunning(pidPath); running {
		client := daemon.NewClient(socketPath)
		a = &app.App{
			Controller: client,
			Events:     daemon.NewDaemonEventSubscriber(socketPath),
			Validator:  schema.NewValidator(),
		}
	} else {
		var err error
		a, err = app.New(ctx, configPath, serialPort)
		if err != nil {
			fatal("failed to initialize: %s", err)
		}
		defer a.Close()
	}

	var err error
	switch args[0] {
	case "health":
		err = cmdHealth(a)
	case "devices":
		err = cmdDevices(ctx, a, args[1:])
	case "discovery":
		err = cmdDiscovery(ctx, a, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		usage()
		os.Exit(1)
	}

	if err != nil {
		fatal("%s", err)
	}
}

func cmdHealth(a *app.App) error {
	status := "healthy"
	controller := "connected"
	if !a.Controller.IsConnected() {
		status = "unhealthy"
		controller = "disconnected"
	}
	return output(map[string]any{
		"status":     status,
		"controller": controller,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	})
}

func cmdDevices(ctx context.Context, a *app.App, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("devices requires a subcommand: list, get, rename, remove, clear, state, set")
	}

	switch args[0] {
	case "list":
		devices, err := a.Controller.ListDevices(ctx)
		if err != nil {
			return fmt.Errorf("list devices: %w", err)
		}
		states := make([]map[string]any, 0, len(devices))
		for i := range devices {
			d := deviceJSON(&devices[i])
			st, err := a.Controller.GetDeviceState(ctx, devices[i].Name)
			if err == nil {
				d["state"] = st
			}
			states = append(states, d)
		}
		return output(map[string]any{"devices": states, "count": len(states)})

	case "get":
		id, _, err := extractID(args[1:])
		if err != nil {
			return err
		}
		d, err := a.Controller.GetDevice(ctx, id)
		if err != nil {
			return fmt.Errorf("get device: %w", err)
		}
		info := deviceJSON(d)
		st, err := a.Controller.GetDeviceState(ctx, d.Name)
		if err == nil {
			info["state"] = st
		}
		return output(map[string]any{"device": info})

	case "rename":
		id, rest, err := extractID(args[1:])
		if err != nil {
			return err
		}
		name := flagValue(rest, "--name")
		if name == "" {
			return fmt.Errorf("--name is required")
		}
		if err := a.Controller.RenameDevice(ctx, id, name); err != nil {
			return fmt.Errorf("rename device: %w", err)
		}
		return output(map[string]any{"success": true, "message": fmt.Sprintf("device %q renamed to %q", id, name)})

	case "remove":
		id, rest, err := extractID(args[1:])
		if err != nil {
			return err
		}
		force := flagPresent(rest, "--force")
		if err := a.Controller.RemoveDevice(ctx, id, force); err != nil {
			return fmt.Errorf("remove device: %w", err)
		}
		return output(map[string]any{"success": true, "message": fmt.Sprintf("device %q removed", id)})

	case "state":
		id, _, err := extractID(args[1:])
		if err != nil {
			return err
		}
		st, err := a.Controller.GetDeviceState(ctx, id)
		if err != nil {
			return fmt.Errorf("get state: %w", err)
		}
		return output(map[string]any{"device": id, "state": st, "timestamp": time.Now().UTC().Format(time.RFC3339)})

	case "set":
		id, rest, err := extractID(args[1:])
		if err != nil {
			return err
		}
		state := flagsToState(rest)
		if len(state) == 0 {
			return fmt.Errorf("at least one state flag is required (e.g. --state ON --brightness 150)")
		}
		// Validate against device schema
		d, err := a.Controller.GetDevice(ctx, id)
		if err != nil {
			return fmt.Errorf("get device: %w", err)
		}
		if err := a.Validator.Validate(d.StateSchema, state); err != nil {
			return fmt.Errorf("validation: %w", err)
		}
		st, err := a.Controller.SetDeviceState(ctx, id, state)
		if err != nil {
			return fmt.Errorf("set state: %w", err)
		}
		return output(map[string]any{"device": id, "state": st, "timestamp": time.Now().UTC().Format(time.RFC3339)})

	case "clear":
		if err := a.Controller.ClearDevices(ctx); err != nil {
			return fmt.Errorf("clear devices: %w", err)
		}
		return output(map[string]any{"success": true, "message": "all devices removed"})

	default:
		return fmt.Errorf("unknown devices subcommand: %s", args[0])
	}
}

func cmdDiscovery(ctx context.Context, a *app.App, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("discovery requires a subcommand: start, stop")
	}

	switch args[0] {
	case "start":
		duration := 120
		if d := flagValue(args[1:], "--duration"); d != "" {
			if _, err := fmt.Sscanf(d, "%d", &duration); err != nil {
				return fmt.Errorf("invalid duration: %s", d)
			}
		}
		waitFor := 0
		if w := flagValue(args[1:], "--wait-for"); w != "" {
			if _, err := fmt.Sscanf(w, "%d", &waitFor); err != nil {
				return fmt.Errorf("invalid wait-for: %s", w)
			}
		}
		if err := a.Controller.PermitJoin(ctx, true, duration); err != nil {
			return fmt.Errorf("start discovery: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Pairing mode enabled for %d seconds. Waiting for devices...\n", duration)

		// Subscribe to discovery events and block until timeout or Ctrl+C.
		ch := a.Events.Subscribe()
		defer a.Events.Unsubscribe(ch)

		timer := time.NewTimer(time.Duration(duration) * time.Second)
		defer timer.Stop()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		defer signal.Stop(sigCh)

		seen := map[string]bool{}
		for {
			select {
			case ev := <-ch:
				if ev.Type == "device_joined" && ev.Device != nil {
					if !seen[ev.Device.ID] {
						seen[ev.Device.ID] = true
						fmt.Fprintf(os.Stderr, "Device joined: %s (%s)\n", ev.Device.ID, ev.Device.Name)
						if waitFor > 0 && len(seen) >= waitFor {
							fmt.Fprintf(os.Stderr, "Reached --wait-for %d. Finishing discovery.\n", waitFor)
							// Give key exchange and cluster discovery time to complete
							time.Sleep(20 * time.Second)
							_ = a.Controller.PermitJoin(ctx, false, 0)
							goto done
						}
					}
				} else if ev.Type == "device_left" && ev.Device != nil {
					fmt.Fprintf(os.Stderr, "Device left: %s\n", ev.Device.ID)
				}
			case <-timer.C:
				fmt.Fprintf(os.Stderr, "Discovery finished. %d device(s) joined.\n", len(seen))
				goto done
			case <-sigCh:
				fmt.Fprintf(os.Stderr, "\nDiscovery interrupted. %d device(s) joined.\n", len(seen))
				_ = a.Controller.PermitJoin(ctx, false, 0)
				goto done
			}
		}
	done:
		devices, err := a.Controller.ListDevices(ctx)
		if err != nil {
			return err
		}
		states := make([]map[string]any, 0, len(devices))
		for i := range devices {
			states = append(states, deviceJSON(&devices[i]))
		}
		return output(map[string]any{"devices": states, "count": len(states), "new_devices": len(seen)})

	case "stop":
		if err := a.Controller.PermitJoin(ctx, false, 0); err != nil {
			return fmt.Errorf("stop discovery: %w", err)
		}
		return output(map[string]any{"success": true, "message": "pairing mode disabled"})

	default:
		return fmt.Errorf("unknown discovery subcommand: %s", args[0])
	}
}

func cmdDaemon(args []string, socketPath, pidPath, logPath, configPath, serialPort string) error {
	if len(args) == 0 {
		return fmt.Errorf("daemon requires a subcommand: start, stop, status")
	}

	switch args[0] {
	case "start":
		if running, pid, _ := daemon.IsRunning(pidPath); running {
			return output(map[string]any{"status": "already running", "pid": pid})
		}
		// Build args for the forked daemon process.
		forkArgs := []string{"--daemon-foreground"}
		if configPath != "" {
			forkArgs = append(forkArgs, "--config", configPath)
		}
		if serialPort != "" {
			forkArgs = append(forkArgs, "--port", serialPort)
		}
		forkArgs = append(forkArgs, "--socket", socketPath, "--pid", pidPath, "--log", logPath)

		if err := daemon.Fork(logPath, forkArgs); err != nil {
			return fmt.Errorf("start daemon: %w", err)
		}
		// Wait for socket to appear (up to 5 seconds).
		for range 50 {
			if _, err := os.Stat(socketPath); err == nil {
				return output(map[string]any{
					"status":  "started",
					"version": version,
					"socket":  socketPath,
					"pid":     pidPath,
					"log":     logPath,
				})
			}
			time.Sleep(100 * time.Millisecond)
		}
		return fmt.Errorf("daemon did not start (check %s for details)", logPath)

	case "stop":
		if err := daemon.StopDaemon(pidPath); err != nil {
			return fmt.Errorf("stop daemon: %w", err)
		}
		return output(map[string]any{"status": "stopped"})

	case "status":
		running, pid, _ := daemon.IsRunning(pidPath)
		if running {
			return output(map[string]any{"status": "running", "version": version, "pid": pid, "socket": socketPath})
		}
		return output(map[string]any{"status": "stopped"})

	default:
		return fmt.Errorf("unknown daemon subcommand: %s (use start, stop, status)", args[0])
	}
}

// --- helpers ---

func deviceJSON(d *device.Device) map[string]any {
	m := map[string]any{
		"ieee_address":  d.ID,
		"friendly_name": d.Name,
		"type":          d.Type,
	}
	if d.Manufacturer != "" {
		m["manufacturer"] = d.Manufacturer
	}
	if d.Model != "" {
		m["model"] = d.Model
	}
	if d.StateSchema != nil {
		m["state_schema"] = d.StateSchema
	}
	return m
}

func extractID(args []string) (string, []string, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "--") {
		return "", nil, fmt.Errorf("device ID is required")
	}
	return args[0], args[1:], nil
}

func flagValue(args []string, key string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == key && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(args[i], key+"=") {
			return args[i][len(key)+1:]
		}
	}
	return ""
}

func flagPresent(args []string, key string) bool {
	for _, a := range args {
		if a == key {
			return true
		}
	}
	return false
}

func flagsToState(args []string) map[string]any {
	state := map[string]any{}
	for i := 0; i < len(args); i++ {
		if !strings.HasPrefix(args[i], "--") {
			continue
		}
		key := strings.TrimPrefix(args[i], "--")
		var val string
		if idx := strings.Index(key, "="); idx != -1 {
			val = key[idx+1:]
			key = key[:idx]
		} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
			i++
			val = args[i]
		} else {
			state[key] = true
			continue
		}

		var n float64
		if _, err := fmt.Sscanf(val, "%f", &n); err == nil {
			if n == float64(int(n)) {
				state[key] = int(n)
			} else {
				state[key] = n
			}
			continue
		}

		switch strings.ToLower(val) {
		case "true":
			state[key] = true
		case "false":
			state[key] = false
		default:
			state[key] = val
		}
	}
	return state
}

func output(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}
	fmt.Println(string(b))
	return nil
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}

func usage() {
	fmt.Fprint(os.Stderr, `Usage: zigbee-skill <command> [args]

Commands:
  health                              Check controller health
  daemon start                        Start background daemon (keeps Zigbee connection alive)
  daemon stop                         Stop the daemon
  daemon status                       Check if daemon is running
  devices list                        List all paired devices
  devices get <id>                    Get device details
  devices rename <id> --name <name>   Rename a device
  devices remove <id> [--force]       Remove a device
  devices clear                       Remove all devices
  devices state <id>                  Get device state
  devices set <id> --state ON         Set device state
  discovery start [--duration 120] [--wait-for 1]  Start pairing mode
  discovery stop                                   Stop pairing mode

Global flags:
  --config <path>   Config file path (default: ./zigbee-skill.yaml)
  --port <path>     Zigbee serial port (overrides config file)
  --socket <path>   Daemon Unix socket (default: /tmp/zigbee-skill.sock)
  --pid <path>      Daemon PID file (default: /tmp/zigbee-skill.pid)
  --log <path>      Daemon log file (default: /tmp/zigbee-skill.log)

When the daemon is running, all commands route through it automatically.
All output is JSON. Pipe to jq for filtering.

Examples:
  zigbee-skill daemon start --port /dev/ttyUSB0
  zigbee-skill daemon status
  zigbee-skill devices list | jq '.devices[].friendly_name'
  zigbee-skill devices set bedroom-lamp --state ON --brightness 150
  zigbee-skill daemon stop
`)
}
