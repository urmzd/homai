package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/urmzd/zigbee-skill/pkg/app"
	"github.com/urmzd/zigbee-skill/pkg/config"
	"github.com/urmzd/zigbee-skill/pkg/daemon"
	"github.com/urmzd/zigbee-skill/pkg/device"
	"github.com/urmzd/zigbee-skill/pkg/device/schema"
	"github.com/urmzd/zigbee-skill/pkg/zigbee"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Set via -ldflags at build time.
var version = "dev"

// Global flags shared across commands.
var (
	configPath      string
	serialPort      string
	socketPath      string
	pidPath         string
	logPath         string
	daemonForeground bool
	noCache         bool
)

// Shared app instance initialised by PersistentPreRunE.
var sharedApp *app.App

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.SetGlobalLevel(zerolog.WarnLevel)

	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

// rootCmd builds and returns the root cobra command.
func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "zigbee-skill",
		Short:         "Zigbee device controller",
		Long:          "zigbee-skill controls Zigbee devices via a local coordinator adapter or a background daemon.",
		SilenceUsage:  true,
		SilenceErrors: true,
		// Internal: run as foreground daemon (called by Fork).
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Handle hidden --daemon-foreground mode.
			if daemonForeground {
				lp := logPath
				if lp == "" {
					lp = daemon.DefaultLogPath
				}
				log.Logger = zerolog.New(&lumberjack.Logger{
					Filename:   lp,
					MaxSize:    10,
					MaxBackups: 2,
					Compress:   true,
				}).With().Timestamp().Logger()
				zerolog.SetGlobalLevel(zerolog.WarnLevel)
				log.Info().Str("version", version).Msg("Starting zigbee-skill daemon")
				sp := socketPath
				if sp == "" {
					sp = daemon.DefaultSocketPath
				}
				pp := pidPath
				if pp == "" {
					pp = daemon.DefaultPIDPath
				}
				srv := daemon.NewServer(sp, pp)
				if err := srv.Start(configPath, serialPort); err != nil {
					return fmt.Errorf("daemon: %s", err)
				}
				os.Exit(0)
			}

			// Commands that don't need the app (daemon/network/update) skip init.
			if skipAppInit(cmd) {
				return nil
			}

			// Fill in default paths.
			if socketPath == "" {
				socketPath = daemon.DefaultSocketPath
			}
			if pidPath == "" {
				pidPath = daemon.DefaultPIDPath
			}
			if logPath == "" {
				logPath = daemon.DefaultLogPath
			}

			ctx := cmd.Context()
			if noCache {
				ctx = device.WithNoCache(ctx)
				// store updated ctx back
				cmd.SetContext(ctx)
			}

			// Auto-detect running daemon and route through it.
			if running, _, _ := daemon.IsRunning(pidPath); running {
				client := daemon.NewClient(socketPath)
				sharedApp = &app.App{
					Controller: client,
					Events:     daemon.NewDaemonEventSubscriber(socketPath),
					Validator:  schema.NewValidator(),
				}
				return nil
			}

			var err error
			sharedApp, err = app.New(ctx, configPath, serialPort)
			if err != nil {
				return fmt.Errorf("failed to initialize: %s", err)
			}
			return nil
		},
	}

	// Persistent global flags.
	pf := root.PersistentFlags()
	pf.StringVar(&configPath, "config", "", "Config file path (default: ./zigbee-skill.yaml)")
	pf.StringVar(&serialPort, "port", "", "Zigbee serial port (overrides config file)")
	pf.StringVar(&socketPath, "socket", daemon.DefaultSocketPath, "Daemon Unix socket")
	pf.StringVar(&pidPath, "pid", daemon.DefaultPIDPath, "Daemon PID file")
	pf.StringVar(&logPath, "log", daemon.DefaultLogPath, "Daemon log file")
	pf.BoolVar(&daemonForeground, "daemon-foreground", false, "Run as foreground daemon (internal)")
	pf.BoolVar(&noCache, "no-cache", false, "Bypass cached device state")
	_ = pf.MarkHidden("daemon-foreground")

	root.AddCommand(
		healthCmd(),
		daemonCmd(),
		devicesCmd(),
		discoveryCmd(),
		networkCmd(),
		updateCmd(),
		versionCmd(),
	)

	return root
}

// skipAppInit returns true for commands that manage their own connection.
func skipAppInit(cmd *cobra.Command) bool {
	// Walk up to find root, collect command path.
	names := []string{}
	for c := cmd; c != nil; c = c.Parent() {
		names = append([]string{c.Name()}, names...)
	}
	if len(names) >= 2 {
		switch names[1] {
		case "daemon", "network", "update", "version":
			return true
		}
	}
	// Also skip if --daemon-foreground is set (handled inline).
	return daemonForeground
}

// --- health ---

func healthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check controller health",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdHealth(sharedApp)
		},
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

// --- daemon ---

func daemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the background daemon",
	}
	cmd.AddCommand(daemonStartCmd(), daemonStopCmd(), daemonStatusCmd())
	return cmd
}

func daemonStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the background daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if socketPath == "" {
				socketPath = daemon.DefaultSocketPath
			}
			if pidPath == "" {
				pidPath = daemon.DefaultPIDPath
			}
			if logPath == "" {
				logPath = daemon.DefaultLogPath
			}
			if running, pid, _ := daemon.IsRunning(pidPath); running {
				return output(map[string]any{"status": "already running", "pid": pid})
			}
			forkArgs := []string{"--daemon-foreground"}
			if configPath != "" {
				forkArgs = append(forkArgs, "--config", configPath)
			}
			if serialPort != "" {
				forkArgs = append(forkArgs, "--port", serialPort)
			}
			forkArgs = append(forkArgs, "--socket", socketPath, "--pid", pidPath, "--log", logPath)

			if err := daemon.Fork(forkArgs); err != nil {
				return fmt.Errorf("start daemon: %w", err)
			}
			for range 300 {
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
		},
	}
}

func daemonStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the background daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if pidPath == "" {
				pidPath = daemon.DefaultPIDPath
			}
			if err := daemon.StopDaemon(pidPath); err != nil {
				return fmt.Errorf("stop daemon: %w", err)
			}
			return output(map[string]any{"status": "stopped"})
		},
	}
}

func daemonStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			if pidPath == "" {
				pidPath = daemon.DefaultPIDPath
			}
			if socketPath == "" {
				socketPath = daemon.DefaultSocketPath
			}
			running, pid, _ := daemon.IsRunning(pidPath)
			if running {
				return output(map[string]any{"status": "running", "version": version, "pid": pid, "socket": socketPath})
			}
			return output(map[string]any{"status": "stopped"})
		},
	}
}

// --- devices ---

func devicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devices",
		Short: "Manage paired devices",
	}
	cmd.AddCommand(
		devicesListCmd(),
		devicesGetCmd(),
		devicesRenameCmd(),
		devicesRemoveCmd(),
		devicesClearCmd(),
		devicesStateCmd(),
		devicesSetCmd(),
	)
	return cmd
}

func devicesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all paired devices",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			devices, err := sharedApp.Controller.ListDevices(ctx)
			if err != nil {
				return fmt.Errorf("list devices: %w", err)
			}
			states := make([]map[string]any, 0, len(devices))
			for i := range devices {
				d := deviceJSON(&devices[i])
				st, err := sharedApp.Controller.GetDeviceState(ctx, devices[i].Name)
				if err == nil {
					d["state"] = st
				}
				states = append(states, d)
			}
			return output(map[string]any{"devices": states, "count": len(states)})
		},
	}
}

func devicesGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <name>",
		Short: "Get device details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			d, err := sharedApp.Controller.GetDevice(ctx, args[0])
			if err != nil {
				return fmt.Errorf("get device: %w", err)
			}
			info := deviceJSON(d)
			st, err := sharedApp.Controller.GetDeviceState(ctx, d.Name)
			if err == nil {
				info["state"] = st
			}
			return output(map[string]any{"device": info})
		},
	}
}

func devicesRenameCmd() *cobra.Command {
	var newName string
	cmd := &cobra.Command{
		Use:   "rename <old>",
		Short: "Rename a device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if newName == "" {
				return fmt.Errorf("--name is required")
			}
			ctx := cmd.Context()
			if err := sharedApp.Controller.RenameDevice(ctx, args[0], newName); err != nil {
				return fmt.Errorf("rename device: %w", err)
			}
			return output(map[string]any{"success": true, "message": fmt.Sprintf("device %q renamed to %q", args[0], newName)})
		},
	}
	cmd.Flags().StringVar(&newName, "name", "", "New device name (required)")
	return cmd
}

func devicesRemoveCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := sharedApp.Controller.RemoveDevice(ctx, args[0], force); err != nil {
				return fmt.Errorf("remove device: %w", err)
			}
			return output(map[string]any{"success": true, "message": fmt.Sprintf("device %q removed", args[0])})
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Force removal")
	return cmd
}

func devicesClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Remove all devices",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := sharedApp.Controller.ClearDevices(ctx); err != nil {
				return fmt.Errorf("clear devices: %w", err)
			}
			return output(map[string]any{"success": true, "message": "all devices removed"})
		},
	}
}

func devicesStateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "state <name>",
		Short: "Get device state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			st, err := sharedApp.Controller.GetDeviceState(ctx, args[0])
			if err != nil {
				return fmt.Errorf("get state: %w", err)
			}
			return output(map[string]any{"device": args[0], "state": st, "timestamp": time.Now().UTC().Format(time.RFC3339)})
		},
	}
}

func devicesSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "set <name> [--key value ...]",
		Short:              "Set device state",
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("device name is required")
			}
			id := args[0]
			rest := args[1:]
			state := flagsToState(rest)
			if len(state) == 0 {
				return fmt.Errorf("at least one state flag is required (e.g. --state ON --brightness 150)")
			}
			ctx := cmd.Context()
			d, err := sharedApp.Controller.GetDevice(ctx, id)
			if err != nil {
				return fmt.Errorf("get device: %w", err)
			}
			if err := sharedApp.Validator.Validate(d.StateSchema, state); err != nil {
				return fmt.Errorf("validation: %w", err)
			}
			st, err := sharedApp.Controller.SetDeviceState(ctx, id, state)
			if err != nil {
				return fmt.Errorf("set state: %w", err)
			}
			return output(map[string]any{"device": id, "state": st, "timestamp": time.Now().UTC().Format(time.RFC3339)})
		},
	}
}

// --- discovery ---

func discoveryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discovery",
		Short: "Manage device discovery / pairing mode",
	}
	cmd.AddCommand(discoveryStartCmd(), discoveryStopCmd())
	return cmd
}

func discoveryStartCmd() *cobra.Command {
	var duration int
	var waitFor int
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start pairing mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := sharedApp.Controller.PermitJoin(ctx, true, duration); err != nil {
				return fmt.Errorf("start discovery: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Pairing mode enabled for %d seconds. Waiting for devices...\n", duration)

			ch := sharedApp.Events.Subscribe()
			defer sharedApp.Events.Unsubscribe(ch)

			timer := time.NewTimer(time.Duration(duration) * time.Second)
			defer timer.Stop()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt)
			defer signal.Stop(sigCh)

			seen := map[string]bool{}
		loop:
			for {
				select {
				case ev := <-ch:
					if ev.Type == "device_joined" && ev.Device != nil {
						if !seen[ev.Device.ID] {
							seen[ev.Device.ID] = true
							fmt.Fprintf(os.Stderr, "Device joined: %s (%s)\n", ev.Device.ID, ev.Device.Name)
							if waitFor > 0 && len(seen) >= waitFor {
								fmt.Fprintf(os.Stderr, "Reached --wait-for %d. Finishing discovery.\n", waitFor)
								time.Sleep(20 * time.Second)
								_ = sharedApp.Controller.PermitJoin(ctx, false, 0)
								break loop
							}
						}
					} else if ev.Type == "device_left" && ev.Device != nil {
						fmt.Fprintf(os.Stderr, "Device left: %s\n", ev.Device.ID)
					}
				case <-timer.C:
					fmt.Fprintf(os.Stderr, "Discovery finished. %d device(s) joined.\n", len(seen))
					break loop
				case <-sigCh:
					fmt.Fprintf(os.Stderr, "\nDiscovery interrupted. %d device(s) joined.\n", len(seen))
					_ = sharedApp.Controller.PermitJoin(ctx, false, 0)
					break loop
				}
			}

			devices, err := sharedApp.Controller.ListDevices(ctx)
			if err != nil {
				return err
			}
			states := make([]map[string]any, 0, len(devices))
			for i := range devices {
				states = append(states, deviceJSON(&devices[i]))
			}
			return output(map[string]any{"devices": states, "count": len(states), "new_devices": len(seen)})
		},
	}
	cmd.Flags().IntVar(&duration, "duration", 120, "Pairing window in seconds")
	cmd.Flags().IntVar(&waitFor, "wait-for", 0, "Stop after N devices have joined")
	return cmd
}

func discoveryStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop pairing mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := sharedApp.Controller.PermitJoin(ctx, false, 0); err != nil {
				return fmt.Errorf("stop discovery: %w", err)
			}
			return output(map[string]any{"success": true, "message": "pairing mode disabled"})
		},
	}
}

// --- network ---

func networkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Manage the Zigbee network",
	}
	cmd.AddCommand(networkResetCmd())
	return cmd
}

func networkResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Clear Zigbee network (forms fresh on next start)",
		RunE: func(cmd *cobra.Command, args []string) error {
			port := serialPort
			if port == "" {
				cfg, err := config.Load(configPath)
				if err == nil {
					port = cfg.Serial.Port
				}
			}
			if port == "" {
				return fmt.Errorf("serial port required: use --port or set serial.port in config")
			}
			c, err := zigbee.NewController(port)
			if err != nil {
				return fmt.Errorf("connect to adapter: %w", err)
			}
			defer c.Close()
			if err := c.ResetNetwork(); err != nil {
				return fmt.Errorf("reset network: %w", err)
			}
			return output(map[string]any{
				"success": true,
				"message": "network cleared — a fresh network will be formed on next startup",
			})
		},
	}
}

// --- version ---

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the zigbee-skill version",
		RunE: func(cmd *cobra.Command, args []string) error {
			return output(map[string]any{"version": version})
		},
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
