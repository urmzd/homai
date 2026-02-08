package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleGetHealth(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	controllerStatus := "disconnected"
	if s.controller.IsConnected() {
		controllerStatus = "connected"
	}

	status := "healthy"
	if controllerStatus != "connected" {
		status = "unhealthy"
	}

	out := GetHealthOutput{
		Status:     status,
		Controller: controllerStatus,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}

	return mcp.NewToolResultText(formatJSON(out)), nil
}

func (s *Server) handleListDevices(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	devices, err := s.controller.ListDevices(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list devices: %s", err)), nil
	}

	infos := make([]DeviceInfo, 0, len(devices))
	for i := range devices {
		info := DeviceToInfo(&devices[i])
		// Try to get state for each device
		state, err := s.controller.GetDeviceState(ctx, devices[i].Name)
		if err == nil {
			info.State = state
		}
		infos = append(infos, info)
	}

	out := ListDevicesOutput{
		Devices: infos,
		Count:   len(infos),
	}

	return mcp.NewToolResultText(formatJSON(out)), nil
}

func (s *Server) handleGetDevice(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := requiredString(request, "id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	d, err := s.controller.GetDevice(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("device not found: %s", err)), nil
	}

	info := DeviceToInfo(d)
	state, err := s.controller.GetDeviceState(ctx, d.Name)
	if err == nil {
		info.State = state
	}

	out := GetDeviceOutput{Device: info}
	return mcp.NewToolResultText(formatJSON(out)), nil
}

func (s *Server) handleRenameDevice(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := requiredString(request, "id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	newName, err := requiredString(request, "new_name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := s.controller.RenameDevice(ctx, id, newName); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to rename device: %s", err)), nil
	}

	out := RenameDeviceOutput{
		Success: true,
		Message: fmt.Sprintf("Device %q renamed to %q", id, newName),
	}
	return mcp.NewToolResultText(formatJSON(out)), nil
}

func (s *Server) handleRemoveDevice(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := requiredString(request, "id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	force := request.GetArguments()["force"]
	forceBool := false
	if f, ok := force.(bool); ok {
		forceBool = f
	}

	if err := s.controller.RemoveDevice(ctx, id, forceBool); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to remove device: %s", err)), nil
	}

	out := RemoveDeviceOutput{
		Success: true,
		Message: fmt.Sprintf("Device %q removed from network", id),
	}
	return mcp.NewToolResultText(formatJSON(out)), nil
}

func (s *Server) handleGetDeviceState(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := requiredString(request, "id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	state, err := s.controller.GetDeviceState(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get device state: %s", err)), nil
	}

	out := GetDeviceStateOutput{
		DeviceID: id,
		State:    state,
	}
	return mcp.NewToolResultText(formatJSON(out)), nil
}

func (s *Server) handleSetDeviceState(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := requiredString(request, "id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	args := request.GetArguments()

	// Extract state from args — it can be passed as a nested "state" object or as flat args
	stateMap := map[string]any{}
	if stateRaw, ok := args["state"]; ok {
		if sm, ok := stateRaw.(map[string]any); ok {
			stateMap = sm
		}
	} else {
		// Fall back: use all args except "id" as state properties
		for k, v := range args {
			if k != "id" {
				stateMap[k] = v
			}
		}
	}

	// Validate against device schema if validator is available
	if s.validator != nil {
		d, err := s.controller.GetDevice(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("device not found: %s", err)), nil
		}
		if err := s.validator.Validate(d.StateSchema, stateMap); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("validation error: %s", err)), nil
		}
	}

	state, err := s.controller.SetDeviceState(ctx, id, stateMap)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to set device state: %s", err)), nil
	}

	out := SetDeviceStateOutput{
		DeviceID: id,
		State:    state,
	}
	return mcp.NewToolResultText(formatJSON(out)), nil
}

func (s *Server) handleStartDiscovery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	duration := 120
	if d, ok := request.GetArguments()["duration_seconds"]; ok {
		if df, ok := d.(float64); ok && df > 0 {
			duration = int(df)
		}
	}

	if err := s.controller.PermitJoin(ctx, true, duration); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to start discovery: %s", err)), nil
	}

	out := StartDiscoveryOutput{
		Success:         true,
		Message:         fmt.Sprintf("Pairing mode enabled for %d seconds", duration),
		DurationSeconds: duration,
	}
	return mcp.NewToolResultText(formatJSON(out)), nil
}

func (s *Server) handleStopDiscovery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := s.controller.PermitJoin(ctx, false, 0); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to stop discovery: %s", err)), nil
	}

	out := StopDiscoveryOutput{
		Success: true,
		Message: "Pairing mode disabled",
	}
	return mcp.NewToolResultText(formatJSON(out)), nil
}

func (s *Server) handleTurnOn(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := requiredString(request, "id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := map[string]any{"state": "ON"}

	if b, ok := request.GetArguments()["brightness"]; ok {
		if bf, ok := b.(float64); ok {
			req["brightness"] = bf
		}
	}

	state, err := s.controller.SetDeviceState(ctx, id, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to turn on device: %s", err)), nil
	}

	out := TurnOnOutput{
		DeviceID: id,
		State:    state,
	}
	return mcp.NewToolResultText(formatJSON(out)), nil
}

func (s *Server) handleTurnOff(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := requiredString(request, "id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := map[string]any{"state": "OFF"}

	state, err := s.controller.SetDeviceState(ctx, id, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to turn off device: %s", err)), nil
	}

	out := TurnOffOutput{
		DeviceID: id,
		State:    state,
	}
	return mcp.NewToolResultText(formatJSON(out)), nil
}

// --- helpers ---

func requiredString(request mcp.CallToolRequest, key string) (string, error) {
	args := request.GetArguments()
	v, ok := args[key]
	if !ok || v == nil {
		return "", fmt.Errorf("required parameter %q is missing", key)
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return "", fmt.Errorf("parameter %q must be a non-empty string", key)
	}
	return s, nil
}

func formatJSON(v any) string {
	b, err := encodeJSON(v)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to marshal response: %s"}`, err)
	}
	return string(b)
}

func encodeJSON(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
