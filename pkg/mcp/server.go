package mcp

import (
	"github.com/mark3labs/mcp-go/server"
	"github.com/urmzd/homai/pkg/device"
	"github.com/urmzd/homai/pkg/device/schema"
)

// Server wraps the MCP server with Homai's device control functionality
type Server struct {
	mcpServer  *server.MCPServer
	controller device.Controller
	validator  *schema.Validator
}

// NewServer creates a new MCP server for device control
func NewServer(controller device.Controller, validator *schema.Validator) *Server {
	s := &Server{
		controller: controller,
		validator:  validator,
	}

	// Create MCP server
	s.mcpServer = server.NewMCPServer(
		"homai",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Register all tools
	s.registerTools()

	return s
}

// ServeStdio starts the MCP server using stdio transport
func (s *Server) ServeStdio() error {
	return server.ServeStdio(s.mcpServer)
}
