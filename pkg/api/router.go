package api

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/urmzd/homai/pkg/api/handlers"
	"github.com/urmzd/homai/pkg/device"
	"github.com/urmzd/homai/pkg/device/schema"
)

// Router holds the Gin engine and dependencies
type Router struct {
	engine     *gin.Engine
	controller device.Controller
	subscriber device.EventSubscriber
	validator  *schema.Validator
}

// NewRouter creates a new API router
func NewRouter(controller device.Controller, subscriber device.EventSubscriber, validator *schema.Validator) *Router {
	gin.SetMode(gin.ReleaseMode)

	engine := gin.New()
	SetupMiddleware(engine)

	router := &Router{
		engine:     engine,
		controller: controller,
		subscriber: subscriber,
		validator:  validator,
	}

	router.setupRoutes()

	return router
}

// setupRoutes configures all API routes
func (r *Router) setupRoutes() {
	// Swagger UI
	r.engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.engine.GET("/docs", func(c *gin.Context) {
		c.Redirect(301, "/swagger/index.html")
	})

	// Health check at root
	healthHandler := handlers.NewHealthHandler(r.controller)
	r.engine.GET("/health", healthHandler.Health)

	// API v1 routes
	v1 := r.engine.Group("/api/v1")
	{
		// Health
		v1.GET("/health", healthHandler.Health)

		// Discovery
		discoveryHandler := handlers.NewDiscoveryHandler(r.controller, r.subscriber)
		discovery := v1.Group("/discovery")
		{
			discovery.POST("/start", discoveryHandler.StartDiscovery)
			discovery.POST("/stop", discoveryHandler.StopDiscovery)
			discovery.GET("/events", discoveryHandler.Events)
		}

		// Devices
		devicesHandler := handlers.NewDevicesHandler(r.controller)
		controlHandler := handlers.NewControlHandler(r.controller, r.validator)
		devices := v1.Group("/devices")
		{
			devices.GET("", devicesHandler.ListDevices)
			devices.GET("/:id", devicesHandler.GetDevice)
			devices.PATCH("/:id", devicesHandler.RenameDevice)
			devices.DELETE("/:id", devicesHandler.RemoveDevice)

			// Device state control
			devices.GET("/:id/state", controlHandler.GetState)
			devices.POST("/:id/state", controlHandler.SetState)
		}
	}
}

// Run starts the HTTP server
func (r *Router) Run(addr string) error {
	return r.engine.Run(addr)
}
