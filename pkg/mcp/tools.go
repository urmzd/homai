package mcp

import "github.com/mark3labs/mcp-go/mcp"

// registerTools registers all MCP tools with the server
func (s *Server) registerTools() {
	// Health check
	s.mcpServer.AddTool(
		mcp.NewTool("get_health",
			mcp.WithDescription("Check the health status of the Homai service and device controller connectivity"),
		),
		s.handleGetHealth,
	)

	// List devices
	s.mcpServer.AddTool(
		mcp.NewTool("list_devices",
			mcp.WithDescription("List all paired devices with their current state"),
		),
		s.handleListDevices,
	)

	// Get device
	s.mcpServer.AddTool(
		mcp.NewTool("get_device",
			mcp.WithDescription("Get detailed information about a specific device by ID or friendly name"),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("Device ID (IEEE address) or friendly name"),
			),
		),
		s.handleGetDevice,
	)

	// Rename device
	s.mcpServer.AddTool(
		mcp.NewTool("rename_device",
			mcp.WithDescription("Change a device's friendly name"),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("Device ID (IEEE address) or current friendly name"),
			),
			mcp.WithString("new_name",
				mcp.Required(),
				mcp.Description("New friendly name for the device"),
			),
		),
		s.handleRenameDevice,
	)

	// Remove device
	s.mcpServer.AddTool(
		mcp.NewTool("remove_device",
			mcp.WithDescription("Remove a device from the network"),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("Device ID (IEEE address) or friendly name"),
			),
			mcp.WithBoolean("force",
				mcp.Description("Force removal even if device is unavailable (default false)"),
			),
		),
		s.handleRemoveDevice,
	)

	// Get device state
	s.mcpServer.AddTool(
		mcp.NewTool("get_device_state",
			mcp.WithDescription("Get the current state of a device (power, brightness, etc.)"),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("Device ID (IEEE address) or friendly name"),
			),
		),
		s.handleGetDeviceState,
	)

	// Set device state
	s.mcpServer.AddTool(
		mcp.NewTool("set_device_state",
			mcp.WithDescription("Set the state of a device. Pass device-specific properties validated against the device's schema."),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("Device ID (IEEE address) or friendly name"),
			),
			mcp.WithObject("state",
				mcp.Required(),
				mcp.Description("State properties to set (e.g. {\"state\": \"ON\", \"brightness\": 200})"),
			),
		),
		s.handleSetDeviceState,
	)

	// Start discovery
	s.mcpServer.AddTool(
		mcp.NewTool("start_discovery",
			mcp.WithDescription("Enable pairing mode to allow new devices to join the network"),
			mcp.WithNumber("duration_seconds",
				mcp.Description("How long to enable pairing mode in seconds (default 120)"),
			),
		),
		s.handleStartDiscovery,
	)

	// Stop discovery
	s.mcpServer.AddTool(
		mcp.NewTool("stop_discovery",
			mcp.WithDescription("Disable pairing mode"),
		),
		s.handleStopDiscovery,
	)

	// Turn on (convenience)
	s.mcpServer.AddTool(
		mcp.NewTool("turn_on",
			mcp.WithDescription("Turn on a device, optionally setting brightness"),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("Device ID (IEEE address) or friendly name"),
			),
			mcp.WithNumber("brightness",
				mcp.Description("Brightness level (optional, device-specific range)"),
			),
		),
		s.handleTurnOn,
	)

	// Turn off (convenience)
	s.mcpServer.AddTool(
		mcp.NewTool("turn_off",
			mcp.WithDescription("Turn off a device"),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("Device ID (IEEE address) or friendly name"),
			),
		),
		s.handleTurnOff,
	)
}
